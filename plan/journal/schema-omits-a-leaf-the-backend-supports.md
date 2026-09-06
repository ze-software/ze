# Schema omits a leaf the backend supports

The backend can set the value, the kernel has a field for it, and the YANG
declares no leaf. So the operator cannot choose it and cannot read which value
applies: the kernel's own default decides, silently and invisibly, and no
`show`, no diff and no completion row names it.

This is not an unwired feature. An unwired feature is reachable in the config
and reaches nothing. Here nothing is reachable at all, so there is no wrong
value to find and no error to read. It surfaces only when somebody compares the
schema against a reference implementation, or against the kernel's own
parameter list.

The tell is a config container that declares fewer leaves than its sibling
containers, for a kind whose backend code path is otherwise identical.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-05 | tunnel-ttl-default | the `vxlan` encapsulation case (`internal/component/iface/yang/ze-iface-conf.yang`) | The vxlan case declares `vni` and `port` and nothing else, so a vxlan tunnel has no `ttl` leaf and its outer TTL is whatever the kernel picks. Every sibling IPv4-underlay kind (gre, gretap, ipip, sit) declares one and now defaults it to 64. VyOS, the closest reference CLI, declares the leaf for vxlan too and defaults it to 64: `interfaces_vxlan.xml.in` includes `parameters-ttl.xml.i` (whose own default is 0, "Inherit") and then overrides it with `defaultValue` 64, exactly as `interfaces_tunnel.xml.in` does for the gre and ipip family. Found by reading that file while settling R-3 of this spec | not fixed, and deliberately not: it is a missing leaf rather than a defect in the leaf this spec changed, so folding it into the same commit would cost that commit its single focus. It is small work with a decision inside it, because adding the leaf means choosing the default, and 64 is only obviously right if Ze follows VyOS here as it did for the other four kinds |
