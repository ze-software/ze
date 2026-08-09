# OSPF Architecture

Design rationale for the unified OSPF engine (`internal/plugins/ospf`). One
document per subsystem: the decision taken, what was rejected, the constraint
the code does not state, and the trap that caught somebody.

Wire byte layouts live beside these documents in
`docs/architecture/wire/ospf.md` (OSPFv2) and
`docs/architecture/wire/ospfv3.md` (OSPFv3). Operator-facing configuration is
`docs/guide/ospf.md`.

## Base engine

| Document | Code |
|----------|------|
| `ospf-1-types.md` | `internal/plugins/ospf/types/` |
| `ospf-2-wire.md` | `internal/plugins/ospf/packet/` |
| `ospf-3-ip-transport.md` | `internal/plugins/ospf/transport/` |
| `ospf-4-component-config.md` | `internal/plugins/ospf/config.go`, `register.go` |
| `ospf-5-interface-ism.md` | `internal/plugins/ospf/iface/` |
| `ospf-6-neighbor-nsm.md` | `internal/plugins/ospf/neighbor/` |
| `ospf-7-lsdb-flooding.md` | `internal/plugins/ospf/lsdb/` |
| `ospf-8-spf-rib.md` | `internal/plugins/ospf/spf/`, `spf_wiring.go` |
| `ospf-9-inter-area-abr.md` | `internal/plugins/ospf/spf/interarea.go`, `spf/summary.go` |
| `ospf-10-as-external-asbr.md` | `internal/plugins/ospf/redistribute/`, `default.go` |
| `ospf-11-stub-nssa.md` | `internal/plugins/ospf/nssa.go`, `spf/area_type.go` |
| `ospf-12-auth.md` | `internal/plugins/ospf/auth_keystore.go`, `auth_wiring.go` |
| `ospf-13-cli-diag-interop.md` | `internal/plugins/ospf/cmd_show.go`, `internal/component/web/handler_ospf.go` |

## Address families

| Document | Code |
|----------|------|
| `ospf-af-unify.md` | `internal/plugins/ospf/codec.go`, `afstrategy_v6.go` |
| `ospfv3-1-types.md` | `internal/plugins/ospf/v3/types/` |
| `ospfv3-2-wire.md` | `internal/plugins/ospf/v3/packet/` |
| `ospfv3-3-ipv6-transport.md` | `internal/plugins/ospf/v3/transport/` |
| `ospfv3-5-nssa-redist.md` | `internal/plugins/ospf/origination_v6_nssa.go` |
| `ospfv3-6-interop-coverage.md` | `internal/plugins/ospf/origination_v6_stub.go` |
| `ospf-ext-15-multi-af.md` | `internal/plugins/ospf/register_multiaf.go`, `multiaf.go` |

## Extensions

| Document | Code |
|----------|------|
| `ospf-ext-1-opaque-framework.md` | `internal/plugins/ospf/opaque_registry.go`, `opaque.go` |
| `ospf-ext-2-traffic-engineering.md` | `internal/plugins/ospf/te.go`, `packet/te_lsa.go` |
| `ospf-ext-3-router-information.md` | `internal/plugins/ospf/ri.go`, `packet/ri_tlv.go` |
| `ospf-ext-4-extended-link-prefix.md` | `internal/plugins/ospf/ext.go`, `packet/ext_prefix.go` |
| `ospf-ext-6-ti-lfa.md` | `internal/plugins/ospf/spf/lfa.go`, `spf/tilfa.go` |
| `ospf-ext-7-virtual-links.md` | `internal/plugins/ospf/virtual_link.go` |
| `ospf-ext-9-graceful-restart.md` | `internal/plugins/ospf/gr.go` |
| `ospf-ext-11-ldp-igp-sync.md` | `internal/plugins/ospf/ldp_sync.go`, `spf/cutedge.go` |
| `ospf-ext-12-multi-instance.md` | `internal/plugins/ospf/multi_instance.go` |
| `ospf-ext-14-debug-introspection.md` | `internal/plugins/ospf/inject.go`, `decode_view.go` |
| `ospf-ext-16-ipsec-auth.md` | `internal/plugins/ospf/ipsec_install.go` |
| `bfd-client.md` | `internal/plugins/ospf/bfd_client.go` |
