# Traffic Control

Ze programs per-interface queueing disciplines, classes, and filters from a single
`traffic-control { }` YANG section. The same config is consumed by two backends;
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
traffic-control {
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
```

<!-- source: internal/component/traffic/schema/ze-traffic-control-conf.yang -- YANG schema -->

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

**All filter types are currently rejected under the vpp backend.** The
initial fw-7 implementation programmed DSCP and protocol filters but
the surrounding VPP pipelines were incomplete, so the features would
have been silent no-ops (sessions in a detached classify table, marks
reading an unrecorded DSCP). Per `rules/exact-or-reject.md` the
verifier refuses these at commit until the full pipelines land.

| filter type | vpp | Deferral |
|------------|-----|----------|
| `dscp` | rejected | Needs ingress `QosRecordEnableDisable` + egress map + mark pipeline. Tracked: `spec-fw-7b-vpp-qos-pipeline` |
| `protocol` | rejected | Needs classify-table attachment via `ClassifySetInterfaceIPTable` + correct packet-offset matching + per-interface / per-family table lifecycle. Tracked: `spec-fw-7b-vpp-classify-pipeline` |
| `mark` | rejected | VPP's classifier matches packet-header bytes, not Linux SKB metadata. Tracked: `spec-fw-7b-vpp-metadata-filter` |

Rate limiting without filters is still useful for single-rate use:
one HTB or TBF class with rate/ceil values becomes one VPP policer
on interface egress that enforces the operator's rate. Multi-class
shaping (steering specific traffic to specific rate buckets) requires
the classify pipeline above.

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
