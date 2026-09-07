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

This sets `net.mpls.conf.eth0.input=1`. The global label-table size is the
`net.mpls.platform_labels` sysctl. It defaults to 0, which disables MPLS
entirely, so ze writes the full 20-bit space (1048575) once, immediately before
it programs its first label. An operator value set in the `sysctl {}` block is
never overwritten: only a table reading exactly 0 is repaired.

The Linux kernel must supply MPLS forwarding, either through the `mpls_router`
and `mpls_iptunnel` modules or built in with `CONFIG_MPLS_ROUTING` and
`CONFIG_MPLS_IPTUNNEL`. ze's own appliance kernel builds both in, so it loads no
module. MPLS is an enrolled kernel capability: ze probes for the AF_MPLS table
rather than for the module list, because a built-in kernel lists no module.

When the configuration asks the kernel FIB to forward MPLS and
`/proc/sys/net/mpls/platform_labels` does not exist, `ze doctor` reports
`doctor-mpls-unavailable` at error severity, `ze` refuses to start, and
`ze config validate` fails. There is no operator override. A daemon that
advertises label forwarding it cannot perform blackholes the traffic it
attracted.

A probe ze could not read is a different answer. It reports
`doctor-mpls-unknown` at warning severity and ze starts, because refusing on a
question ze could not ask would stop a router whose kernel is fine.

A VPP or P4 FIB backend is never judged on the kernel's AF_MPLS table. The
`fib { kernel { } }` block is what activates the plugin that programs kernel
labels, and it is that plugin that carries the requirement.

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

`keepalive-time` is the KeepAlive Time ze proposes in the Initialization message
of each new session (RFC 5036 section 3.5.3). The two LSRs keep the lower of the
two proposals. Ze then sends a KeepAlive every third of the negotiated value, and
it closes the session when no PDU arrives inside three times that value. The
proposal is exchanged one time, when the session starts, so a change to this leaf
applies to the sessions that open after it and leaves an established session on
the value it negotiated.

<!-- source: internal/plugins/ldp/session.go -- NewSession, SendInit -->
<!-- source: internal/plugins/ldp/register.go -- sessionConfigForAdj -->

Inspect LDP state with `show ldp neighbor` (session state, transport address)
and `show ldp binding` (FEC-to-label bindings). A label binding learned from a
neighbor programs an ingress push entry in the kernel MPLS FIB toward that
neighbor.

<!-- source: internal/plugins/fib/kernel/mpls.go -- label validation -->
<!-- source: internal/plugins/fib/kernel/nexthop_linux.go -- buildMPLSEncap (push) -->
<!-- source: internal/component/iface/config_sysctl.go -- net.mpls.conf.<iface>.input -->
<!-- source: internal/component/mpls/show_forwarding.go -- show mpls forwarding -->
