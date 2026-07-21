# MPLS

Ze can act as a Linux MPLS label-switching router (LSR). BGP labeled-unicast
routes (RFC 8277) received from peers are programmed into the kernel MPLS FIB as
label-push entries, so Ze forwards labeled traffic without VPP.

## Enabling MPLS

MPLS label processing is per-interface. Enable it on each interface that should
accept labeled packets:

```
interface {
    ethernet eth0 {
        unit 0 {
            mpls {
                enable true
            }
        }
    }
}
```

This sets `net.mpls.conf.eth0.input=1`. The global label-table size is governed
by the `net.mpls.platform_labels` sysctl (configure it via the `sysctl {}`
block; it defaults to off, which disables MPLS entirely).

The Linux kernel must have the `mpls_router` and `mpls_iptunnel` modules
loaded. `ze doctor` warns (`doctor-mpls-unavailable`) when they are missing.

## Inspecting the forwarding table

```
ze show mpls forwarding              # all installed MPLS entries
ze show mpls forwarding limit 500    # cap the response size
```

Each row reports the incoming label (`in-label`), the `operation` (`swap` or
`pop`), any outgoing `out-labels`, the `next-hop`, and the egress `device`. The
data is read directly from the kernel AF_MPLS routing table.

## Metrics

- `ze_fibkernel_mpls_routes_installed` -- current MPLS labeled-route count.
- `ze_fibkernel_mpls_installs_total` -- MPLS routes successfully programmed.

## Label distribution

Two label-distribution protocols are available (both experimental):

- **LDP** (RFC 5036) for IGP-shortest-path label distribution.
- **RSVP-TE** (RFC 3209) for traffic-engineered explicit-path LSPs with bandwidth
  reservation -- see [RSVP-TE](../rsvp-te/index.md).

### LDP configuration

```
ldp {
    lsr-id 10.0.0.1
    transport-address 10.0.0.1
    interfaces eth0
    interfaces eth1
}
```

- `lsr-id` -- this router's LSR identifier (IPv4 address format).
- `transport-address` -- address advertised for the LDP TCP session.
- `interfaces` -- a list entry per interface on which LDP Hello discovery runs
  (repeat the leaf for each interface).
- `hello-interval` / `hello-hold-time` / `keepalive-time` -- optional soft-state
  timers (defaults 5s / 15s / 60s).

Inspect LDP state with `show ldp neighbor` (session state, transport address)
and `show ldp binding` (FEC-to-label bindings). A label binding learned from a
neighbor programs an ingress push entry in the kernel MPLS FIB toward that
neighbor.

<!-- source: internal/plugins/fib/kernel/mpls.go -- label validation -->
<!-- source: internal/plugins/fib/kernel/nexthop_linux.go -- buildMPLSEncap (push) -->
<!-- source: internal/component/iface/config_sysctl.go -- net.mpls.conf.<iface>.input -->
<!-- source: internal/component/mpls/show_forwarding.go -- show mpls forwarding -->
