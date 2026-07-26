# Traffic Control

Ze programs per-interface queueing disciplines, classes, and filters from a single
`traffic { control { } }` YANG section. The same config is consumed by two backends;
the operator chooses one via the `backend` leaf.

<!-- source: internal/component/traffic/register.go -- reactor wiring -->
<!-- source: internal/component/traffic/backend.go -- Backend interface, RegisterBackend, RegisterVerifier -->

## Backends

| Backend | Platform | Default | Mechanism |
|---------|----------|---------|-----------|
| `tc` | Linux | yes | vishvananda/netlink tc calls (HTB, HFSC, FQ, TBF, netem, ...) |
| `vpp` | Linux with VPP | no | GoVPP binary API (policers, QoS egress maps, classifier sessions) |

Selection:

```
traffic {
    control {
        backend tc       // or: vpp
        interface eth0 {
            qdisc {
                type htb
                class fast {
                    rate 10mbit
                    ceil 20mbit
                }
            }
        }
    }
}
```

<!-- source: internal/component/traffic/yang/ze-traffic-control-conf.yang -- YANG schema -->

## tc Backend: Original Qdisc Snapshot

Before installing its own root qdisc, the tc backend snapshots the interface's existing one so the interface can be restored exactly. A qdisc whose parameters this backend cannot reproduce is refused rather than approximated, and the apply fails naming it (`exact-or-reject.md`).

`noqueue` is snapshotted and restored. It is the default root on every virtual interface (veth, dummy, bridge, and anything else the kernel gives no real queue), so it is the state a QoS config is most often applied *from*. It carries no reconstructable parameters, but it is the *absence* of a discipline, so it is exactly restorable: the restore deletes whatever root Ze installed rather than replacing it, which returns the interface to `noqueue`. Adding a qdisc named `noqueue` is not the inverse operation, and a snapshot restored by deletion fails closed if a caller routes it down the replace path.

Other `GenericQdisc` types (`mq`, `clsact`, ...) stay rejected: those carry state this backend cannot reproduce.
<!-- source: internal/plugins/traffic/netlink/snapshot_linux.go -- newQdiscSnapshot, tcQdiscSnapshot.restoredByDelete -->

## tc Backend: Filter Priorities

The kernel keeps exactly one `tcf_proto` per (parent, priority), and that instance carries a single link-layer protocol (`tcf_chain_tp_find`, `net/sched/cls_api.c`). A second filter at the same priority with a different protocol is rejected with `EINVAL`.

Ze therefore allocates one priority per link-layer protocol: the IPv4 (`ETH_P_IP`) and IPv6 (`ETH_P_IPV6`) halves of a DSCP or protocol match get distinct priorities, as does a mark filter (`ETH_P_ALL`). With a shared priority the qdisc and class were created, the IPv4 filter was accepted, and the IPv6 filter at the same priority was refused.
<!-- source: internal/plugins/traffic/netlink/translate_linux.go -- u32FilterPair, FilterMark -->

## VPP Backend: Compatibility Matrix

The VPP backend rejects qdisc and filter types that cannot be represented
faithfully in VPP. Rejection fires at `ze config commit` (daemon config-verify)
with a message of the form `<type>: not supported by backend vpp`, so operators
learn about incompatibilities before the config lands rather than after Apply.

| qdisc type | vpp | Notes |
|-----------|-----|-------|
| `htb` with exactly 1 class | accepted | One policer: CIR = Rate kbps, EIR = Ceil kbps (2R3C RFC 2698), bound to interface egress via `PolicerOutput` |
| `tbf` with exactly 1 class | accepted | One policer: CIR = EIR = Rate kbps (1R2C), bound to interface egress via `PolicerOutput` |
| `htb` / `tbf` with 0 or >1 classes | rejected | Multi-class shaping needs filter-based classification (deferred). Without filters, every class's policer would stack on VPP's output feature arc in series; effective rate becomes `min(class_rates)` rather than per-class shaping. |
| `prio` | rejected | The class-index to DSCP-value mapping needs an explicit design (deferred) |
| `hfsc` | rejected | Service-curve semantics have no VPP equivalent |
| `fq` / `sfq` / `fq_codel` | rejected | Fair-queue disciplines not available in VPP |
| `netem` | rejected | Network emulation not available in VPP |
| `clsact` / `ingress` | rejected | Ingress policing semantics differ in VPP (deferred) |

**`protocol` filters are supported; `dscp` and `mark` are rejected.** A
single class may carry `match protocol <n>`: the backend then builds per-family
(IPv4 + IPv6) classify tables, adds a session per protocol steering to the
class policer (`ClassifyAddDelSession.HitNextIndex` = policer index, matching
the packet at absolute frame offset 23 for IPv4 protocol / 20 for IPv6
next-header), and binds the tables to the interface's `policer-classify`
feature so only matching traffic is policed. The initial fw-7 attempt matched
the wrong offset and never attached the table (silent no-op); the current
pipeline is golden-vector tested and validated against real VPP v25.10. DSCP
and mark stay rejected per `rules/exact-or-reject.md` until their pipelines
land.

| filter type | vpp | Notes |
|------------|-----|----------|
| `protocol` | accepted | Per-family classify tables bound to `policer-classify`; only matching traffic is policed (ingress). Values 0-255. |
| `dscp` | rejected | Needs ingress `QosRecordEnableDisable` + egress map + mark pipeline. |
| `mark` | rejected | VPP's classifier matches packet-header bytes, not Linux SKB metadata. |

Rate limiting without filters is still useful for single-rate use:
one HTB or TBF class with rate/ceil values becomes one VPP policer
on interface egress that enforces the operator's rate. Multi-class
shaping (steering specific traffic to distinct rate buckets) is not yet
supported under the vpp backend.

Policer name length: the backend composes a VPP policer name as
`ze/<iface>/<class>` and VPP caps names at 64 bytes. If that compound
exceeds 64 bytes, the verifier rejects the commit with a message naming
the full name and the limit so the operator can shorten the class or
interface name. No silent truncation -- two distinct classes must never
produce the same stored policer name.

<!-- source: internal/plugins/traffic/vpp/verify.go -- rejection matrix -->
<!-- source: internal/plugins/traffic/vpp/translate.go -- parameter translation -->

## VPP Backend: Operational Notes

- **VPP must be running when traffic-control config applies.** If VPP is not
  reachable, `Apply` waits up to 5 seconds for the GoVPP connection and then
  returns `vpp not connected after 5s`. The commit fails and the operator is
  expected to start VPP and retry. There is no silent fallback.
- **Rates round up.** Ze uses bps internally; VPP uses kbps. `1500 bps`
  becomes `2 kbps` at the VPP layer. Rates above approximately 4.3 Tbps
  overflow VPP's `uint32` kbps and are rejected explicitly.
- **No reconciliation by the backend.** The traffic component tracks the
  previous config and invokes `Apply` with the new full desired state on
  every reload. The backend diffs against the last applied policers and
  unbinds/deletes ones no longer referenced.
- **Interface must exist in VPP.** Names come from the config and are looked
  up via `SwInterfaceDump` each apply. An unknown interface name fails the
  apply with `interface "<name>" not present in vpp`.

<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- Apply, WaitConnected, reconcileRemovals -->
<!-- source: internal/component/vpp/conn.go -- Connector.WaitConnected -->

## Failure Modes

| Symptom | Likely cause | Resolution |
|--------|-------------|------------|
| Commit fails with `<type>: not supported by backend vpp` | Config uses a qdisc/filter rejected by the vpp backend | Change the qdisc/filter to one from the accepted list, or switch to `backend tc` |
| Commit fails with `vpp not connected after 5s` | VPP daemon not running or unreachable | Start VPP, wait for its API socket to be ready, retry commit |
| Commit fails with `interface "<name>" not present in vpp` | Interface declared in traffic-control config is unknown to VPP | Create the interface in VPP first (via the `interface` component or manually), then retry |
