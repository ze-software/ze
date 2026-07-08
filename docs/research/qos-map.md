  802.1p / PCP QoS Maps: Vendor Configuration Reference

  1. Juniper JunOS (MX/EX)

  Terminology

  JunOS uses class-of-service as the top-level hierarchy. Internal priority is a forwarding class (e.g., best-effort, expedited-forwarding, assured-forwarding, network-control, or custom
  names). PCP values are expressed as 3-bit binary code-points (000, 101, etc.). Each mapping entry is a compound key of (forwarding-class, loss-priority).

  Architecture

  Two-level indirection through named table objects: define a classifier (ingress) and a rewrite rule (egress), then bind both to an interface unit. This is a define-then-apply pattern.
  Not a flat map.

  Ingress: PCP to Forwarding Class (Classifier)

  class-of-service {
      classifiers {
          ieee-802.1 my-classifier {
              forwarding-class best-effort {
                  loss-priority low code-points 000;
                  loss-priority high code-points 001;
              }
              forwarding-class expedited-forwarding {
                  loss-priority low code-points 101;
              }
              forwarding-class network-control {
                  loss-priority low code-points 110;
                  loss-priority high code-points 111;
              }
          }
      }
  }

  Egress: Forwarding Class to PCP (Rewrite Rule)

  class-of-service {
      rewrite-rules {
          ieee-802.1 my-rewrite {
              forwarding-class best-effort {
                  loss-priority low code-point 000;
                  loss-priority high code-point 001;
              }
              forwarding-class expedited-forwarding {
                  loss-priority low code-point 101;
              }
          }
      }
  }

  Binding to Interface

  class-of-service {
      interfaces {
          ge-0/0/0 {
              unit 100 {
                  classifiers {
                      ieee-802.1 my-classifier;
                  }
                  rewrite-rules {
                      ieee-802.1 my-rewrite;
                  }
              }
          }
      }
  }

  Summary

  ┌─────────────┬─────────────────────────────────────────────────────────────────────────────────────────────────────────┐
  │   Aspect    │                                                 Detail                                                  │
  ├─────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Scope       │ Named tables defined globally, bound per-unit (per VLAN sub-interface)                                  │
  ├─────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Ingress     │ classifiers { ieee-802.1 <name> { forwarding-class <fc> { loss-priority <lp> code-points <bits>; } } }  │
  ├─────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Egress      │ rewrite-rules { ieee-802.1 <name> { forwarding-class <fc> { loss-priority <lp> code-point <bits>; } } } │
  ├─────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Map shape   │ Compound key: (forwarding-class, loss-priority) maps to code-point. Not flat PCP:PCP.                   │
  ├─────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ QinQ        │ ieee-802.1ad variant for dual-tag, adds DEI bit handling with 4-bit code-points                         │
  ├─────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Inheritance │ Can apply at physical interface level; unit-level overrides                                             │
  ├─────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Gotcha      │ Once a rewrite rule is applied to any FC on a port, all FCs on that port need rewrite rules             │
  └─────────────┴─────────────────────────────────────────────────────────────────────────────────────────────────────────┘

  ---
  2. Cisco IOS / IOS-XE

  Terminology

  Cisco calls PCP CoS (Class of Service). The 3-bit field is referred to interchangeably as "CoS", "802.1p", or "User Priority" (values 0-7). Two distinct configuration eras exist.

  Legacy Model: MLS QoS (Catalyst 3560/3750, IOS 12.x-15.x)

  Global flat maps with per-interface trust selection:

  ! Enable QoS globally
  mls qos

  ! CoS-to-DSCP map (8 positional DSCP values for CoS 0-7)
  mls qos map cos-dscp 0 8 16 24 32 46 48 56

  ! DSCP-to-CoS map
  mls qos map dscp-cos 0 8 10 16 18 24 26 to 3
  mls qos map dscp-cos 46 to 5

  ! Per-interface trust
  interface GigabitEthernet1/0/1
    mls qos trust cos
    mls qos cos 4          ! override default port CoS

  This model is global (one cos-dscp and dscp-cos table for the whole switch). No intermediate abstractions. CoS maps through DSCP, not directly CoS-to-CoS.

  Modern Model: MQC / Table-Map (Catalyst 9000, IOS-XE 16.x/17.x)

  Uses Modular QoS CLI with class-map/policy-map/table-map:

  ! Reusable mapping table
  table-map cos-remap
    map from 0 to 0
    map from 1 to 1
    map from 5 to 5
    default copy

  ! Classification
  class-map match-any VOICE
    match cos 5

  ! Marking policy
  policy-map MARK-EGRESS
    class VOICE
      set cos 5
    class class-default
      set cos 0

  ! Table-map based ingress trust
  policy-map INGRESS-TRUST
    class class-default
      set cos cos table cos-remap

  ! Apply to sub-interface
  interface GigabitEthernet1/0/1.100
    encapsulation dot1Q 100
    service-policy input INGRESS-TRUST
    service-policy output MARK-EGRESS

  Summary

  ┌───────────┬───────────────────────────────────────────┬───────────────────────────────────────────┐
  │  Aspect   │             MLS QoS (legacy)              │         MQC / Table-Map (modern)          │
  ├───────────┼───────────────────────────────────────────┼───────────────────────────────────────────┤
  │ Scope     │ Global map, per-interface trust           │ Named table/policy, per-interface binding │
  ├───────────┼───────────────────────────────────────────┼───────────────────────────────────────────┤
  │ Ingress   │ mls qos trust cos + global cos-dscp table │ set cos cos table <name> in policy-map    │
  ├───────────┼───────────────────────────────────────────┼───────────────────────────────────────────┤
  │ Egress    │ DSCP-to-CoS global table                  │ set cos <value> in policy-map class       │
  ├───────────┼───────────────────────────────────────────┼───────────────────────────────────────────┤
  │ Map shape │ Flat positional (CoS goes through DSCP)   │ map from X to Y in table-map              │
  ├───────────┼───────────────────────────────────────────┼───────────────────────────────────────────┤
  │ Note      │ Switch-only, no sub-interface granularity │ Routers (ISR, ASR) use MQC exclusively    │
  └───────────┴───────────────────────────────────────────┴───────────────────────────────────────────┘

  ---
  3. Cisco IOS-XR

  Terminology

  IOS-XR uses class-map, policy-map, match cos, set cos, and an internal-only qos-group (0-7) that bridges ingress classification to egress remarking. There is no flat map; everything goes
  through the three-layer MQC hierarchy.

  Architecture: Three-Step Indirect Model

  1. Ingress policy classifies with match cos and sets qos-group
  2. qos-group travels through the forwarding path (not a wire field)
  3. Egress policy matches qos-group and writes PCP with set cos

  Ingress: PCP to Internal Priority

  class-map match-any cos5
    match cos 5
    end-class-map
  !
  class-map match-any cos3
    match cos 3
    end-class-map
  !
  policy-map ingress-classify
    class cos5
      set qos-group 5
    !
    class cos3
      set qos-group 3
    !
    class class-default
      set qos-group 0
    !
    end-policy-map

  Egress: Internal Priority to PCP

  class-map match-any qos5
    match qos-group 5
    end-class-map
  !
  policy-map egress-marking
    class qos5
      set cos 5
    !
    class class-default
      set cos 0
    !
    end-policy-map

  Apply to Sub-Interface

  interface GigabitEthernet0/0/0/0.100 l2transport
    encapsulation dot1q 100
    service-policy input ingress-classify
    service-policy output egress-marking

  Summary

  ┌─────────────┬──────────────────────────────────────────────────────────────────────────────┐
  │   Aspect    │                                    Detail                                    │
  ├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
  │ Scope       │ Per-interface, per-direction. No global CoS map.                             │
  ├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
  │ Ingress     │ match cos <0-7> + set qos-group <0-7> in ingress policy-map                  │
  ├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
  │ Egress      │ match qos-group <0-7> + set cos <0-7> in egress policy-map                   │
  ├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
  │ Map shape   │ Multi-layer abstraction: class-map + policy-map + qos-group indirection      │
  ├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
  │ Constraints │ match qos-group is egress-only. Cannot set cos at ingress and have it stick. │
  ├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
  │ DEI support │ set dei <0-1> available alongside set cos                                    │
  ├─────────────┼──────────────────────────────────────────────────────────────────────────────┤
  │ L2 mode     │ l2transport sub-interface mode required for L2VPN/VPLS scenarios             │
  └─────────────┴──────────────────────────────────────────────────────────────────────────────┘

  ---
  4. Nokia SR OS

  Terminology

  Nokia uses SAP (Service Access Point) for the VLAN attachment point (format: port:vlan, e.g., 1/1/1:100). Internal priority is a forwarding class (fc): eight built-in classes (be, l2,
  af, l1, h2, ef, h1, nc). PCP is called dot1p. QoS is defined in numbered sap-ingress and sap-egress policies, then applied per-SAP.

  Architecture

  Two-level indirection: dot1p -> fc -> queue. Policies are defined centrally with a numeric ID, then referenced by SAPs across services.

  Ingress: PCP to Forwarding Class

  configure qos sap-ingress 100 create
      default-fc be
      default-priority low
      dot1p 0 fc be priority low
      dot1p 1 fc l2 priority low
      dot1p 5 fc ef priority high
      dot1p 7 fc nc priority high
  exit

  Each dot1p <value> fc <name> [priority {low|high}] maps one PCP value to an internal forwarding class.

  Egress: Forwarding Class to PCP

  configure qos sap-egress 200 create
      fc be create
          dot1p 0
      exit
      fc ef create
          dot1p in-profile 5 out-profile 3
      exit
      fc nc create
          dot1p 7
      exit
  exit

  Remarking is per-fc, with optional in-profile/out-profile/exceed-profile differentiation. For QinQ, dot1p-inner and dot1p-outer remark tags independently.

  Apply to SAP

  configure service epipe 1000 customer 1 create
      sap 1/1/1:100 create
          ingress
              qos 100
          exit
          egress
              qos 200
          exit
      exit
  exit

  Summary

  ┌───────────┬─────────────────────────────────────────────────────────────┐
  │  Aspect   │                           Detail                            │
  ├───────────┼─────────────────────────────────────────────────────────────┤
  │ Scope     │ Per-policy (numbered), applied per-SAP within a service     │
  ├───────────┼─────────────────────────────────────────────────────────────┤
  │ Ingress   │ dot1p <val> fc <name> (flat map, 8 entries max)             │
  ├───────────┼─────────────────────────────────────────────────────────────┤
  │ Egress    │ fc <name> { dot1p <val> } (reverse map, profile-aware)      │
  ├───────────┼─────────────────────────────────────────────────────────────┤
  │ Map shape │ Two-level: dot1p <-> forwarding-class <-> queue             │
  ├───────────┼─────────────────────────────────────────────────────────────┤
  │ QinQ      │ dot1p-inner, dot1p-outer for independent tag remarking      │
  ├───────────┼─────────────────────────────────────────────────────────────┤
  │ Reuse     │ Policies can be template (shared) or exclusive (single SAP) │
  └───────────┴─────────────────────────────────────────────────────────────┘

  ---
  5. Arista EOS

  Terminology

  Arista uses CoS for the PCP field and traffic-class (TC) for internal priority. Both are 0-7. The two maps are called cos-tc (ingress) and tc-cos (egress).

  Architecture

  The simplest model of all vendors. Flat, global, 8-entry lookup tables. No intermediate abstractions for the base map.

  Ingress: CoS to Traffic-Class

  qos map cos 0 to traffic-class 0
  qos map cos 1 3 5 7 to traffic-class 5
  qos map cos 2 to traffic-class 2
  qos map cos 4 to traffic-class 4
  qos map cos 6 to traffic-class 6

  Multiple CoS values can map to a single traffic-class in one command.

  Egress: Traffic-Class to CoS

  qos map traffic-class 0 to cos 0
  qos map traffic-class 1 3 5 to cos 2
  qos map traffic-class 4 to cos 4
  qos map traffic-class 6 to cos 6
  qos map traffic-class 7 to cos 7

  Trust Mode (per-interface)

  interface Ethernet1
    qos trust cos

  The port must trust CoS for the cos-tc map to take effect. Sub-interfaces inherit from the parent.

  Summary

  ┌───────────────┬─────────────────────────────────────────────────────────────────────────────────────────────┐
  │    Aspect     │                                           Detail                                            │
  ├───────────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Scope         │ Global for the map tables. Trust mode is per-interface.                                     │
  ├───────────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Ingress       │ qos map cos <vals> to traffic-class <val>                                                   │
  ├───────────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Egress        │ qos map traffic-class <vals> to cos <val>                                                   │
  ├───────────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Map shape     │ Flat key:value, 8 entries, no intermediate abstractions                                     │
  ├───────────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Sub-interface │ Maps are global, not per-VLAN. Per-flow overrides via policy-map set cos/set traffic-class. │
  ├───────────────┼─────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Limitation    │ QoS profiles (qos profile) not supported on SVIs or VLAN sub-interfaces                     │
  └───────────────┴─────────────────────────────────────────────────────────────────────────────────────────────┘

  ---
  6. Linux (iproute2)

  Terminology

  Linux uses skb->priority as the internal priority (set by SO_PRIORITY socket option or tc actions). PCP is the 3-bit field in the 802.1Q tag. Maps are called ingress-qos-map and
  egress-qos-map.

  Architecture

  Flat key:value maps, per-VLAN-interface. No abstractions. The most direct model.

  Creating with Maps

  ip link add link eth0 name eth0.100 type vlan id 100 \
      ingress-qos-map 0:0 1:1 2:2 3:3 4:4 5:5 6:6 7:7 \
      egress-qos-map 0:0 1:1 2:2 3:3 4:4 5:5 6:6 7:7

  Modifying Existing

  ip link set eth0.100 type vlan egress-qos-map 4:5 6:6 7:7
  ip link set eth0.100 type vlan ingress-qos-map 5:4

  FROM:TO Semantics

  ┌─────────────────┬──────────────────────────────────────────────────┬────────────────────────────────────────────────────────┐
  │    Direction    │                       FROM                       │                           TO                           │
  ├─────────────────┼──────────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ ingress-qos-map │ 802.1p PCP value (0-7) from incoming VLAN header │ Linux skb->priority                                    │
  ├─────────────────┼──────────────────────────────────────────────────┼────────────────────────────────────────────────────────┤
  │ egress-qos-map  │ Linux skb->priority                              │ 802.1p PCP value (0-7) written to outgoing VLAN header │
  └─────────────────┴──────────────────────────────────────────────────┴────────────────────────────────────────────────────────┘

  Inspection

  cat /proc/net/vlan/eth0.100      # shows INGRESS/EGRESS priority mappings
  ip -d link show eth0.100         # also displays current maps

  tc Integration

  tc can set skb->priority before the egress map applies, composing the two mechanisms:

  tc qdisc add dev eth0.100 root handle 1: prio
  tc filter add dev eth0.100 parent 1: protocol ip u32 \
      match ip tos 0xb8 0xfc action skbedit priority 5

  Summary

  ┌───────────────────┬───────────────────────────────────────────────────────────────────────────────┐
  │      Aspect       │                                    Detail                                     │
  ├───────────────────┼───────────────────────────────────────────────────────────────────────────────┤
  │ Scope             │ Per-VLAN-interface                                                            │
  ├───────────────────┼───────────────────────────────────────────────────────────────────────────────┤
  │ Ingress           │ ingress-qos-map PCP:skb_priority                                              │
  ├───────────────────┼───────────────────────────────────────────────────────────────────────────────┤
  │ Egress            │ egress-qos-map skb_priority:PCP                                               │
  ├───────────────────┼───────────────────────────────────────────────────────────────────────────────┤
  │ Map shape         │ Flat key:value pairs, no abstractions                                         │
  ├───────────────────┼───────────────────────────────────────────────────────────────────────────────┤
  │ Kernel structures │ ingress_priority_map[8] (fixed array), egress_priority_map (hash of mappings) │
  ├───────────────────┼───────────────────────────────────────────────────────────────────────────────┤
  │ Defaults          │ All zeros (PCP 0 on egress, priority 0 on ingress)                            │
  └───────────────────┴───────────────────────────────────────────────────────────────────────────────┘

  ---
  Cross-Vendor Comparison

  ┌────────────┬───────────────────────────┬──────────────────────────┬──────────────────────────────────────┬─────────────────────────────────────┬─────────────────────────────────┐
  │   Vendor   │       Ingress term        │       Egress term        │         Internal abstraction         │                Scope                │         Map complexity          │
  ├────────────┼───────────────────────────┼──────────────────────────┼──────────────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────┤
  │ Juniper    │ classifier ieee-802.1     │ rewrite-rule ieee-802.1  │ forwarding-class + loss-priority     │ Per-unit (define globally, bind     │ Compound key, named tables      │
  │            │                           │                          │                                      │ per-unit)                           │                                 │
  ├────────────┼───────────────────────────┼──────────────────────────┼──────────────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────┤
  │ Cisco      │ mls qos trust cos /       │ mls qos map / set cos    │ CoS via DSCP (legacy) or table-map   │ Global (legacy) or per-interface    │ Flat (legacy) or named table    │
  │ IOS/XE     │ table-map                 │                          │ (modern)                             │ (MQC)                               │ (modern)                        │
  ├────────────┼───────────────────────────┼──────────────────────────┼──────────────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────┤
  │ Cisco      │ match cos + set qos-group │ match qos-group + set    │ qos-group (0-7)                      │ Per-interface, per-direction        │ Three-layer MQC, most verbose   │
  │ IOS-XR     │                           │ cos                      │                                      │                                     │                                 │
  ├────────────┼───────────────────────────┼──────────────────────────┼──────────────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────┤
  │ Nokia SR   │ sap-ingress dot1p fc      │ sap-egress fc dot1p      │ forwarding-class (8 named)           │ Per-policy, applied per-SAP         │ Two-level, profile-aware        │
  │ OS         │                           │                          │                                      │                                     │                                 │
  ├────────────┼───────────────────────────┼──────────────────────────┼──────────────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────┤
  │ Arista EOS │ qos map cos to            │ qos map traffic-class to │ traffic-class (0-7)                  │ Global maps, per-interface trust    │ Flat, simplest vendor model     │
  │            │ traffic-class             │  cos                     │                                      │                                     │                                 │
  ├────────────┼───────────────────────────┼──────────────────────────┼──────────────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────┤
  │ Linux      │ ingress-qos-map PCP:prio  │ egress-qos-map prio:PCP  │ skb->priority                        │ Per-VLAN-interface                  │ Flat key:value, no abstractions │
  └────────────┴───────────────────────────┴──────────────────────────┴──────────────────────────────────────┴─────────────────────────────────────┴─────────────────────────────────┘
