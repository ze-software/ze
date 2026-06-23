# 958 -- ospf-4-component-config

## Context

OSPFv2 needed the same config-to-engine backbone that IS-IS already has: a self-contained edge plugin, YANG config root, SDK lifecycle callbacks, transport enrolment, and functional config validation. Before this child, the OSPF wire and transport packages existed, but no top-level `ospf {}` config could start an engine or enrol interfaces. The goal was to create only the wiring skeleton that later ISM, NSM, LSDB, SPF, auth, and CLI specs fill in.

## Decisions

- Chose `internal/plugins/ospf/` over a central component package because the user contract and IS-IS/LDP pattern keep protocol runtime, YANG, docs anchors, and tests self-contained.
- Chose per-interface area binding over FRR-style `network <prefix> area` because Ze config should name the interface that owns OSPF state and avoid hidden prefix matching.
- Chose central OSPF router-id and area-id validators over component-local validators because the config package owns custom YANG validation and cannot import the OSPF plugin without a cycle.
- Chose dispatcher validation by receiving interface area over declared-area-only validation because later ISM, NSM, and LSDB code must never see packets from the wrong area on a valid interface.
- Chose parser-side IPv4 range enforcement in addition to YANG `zt:prefix-ipv4` because unit tests and SDK paths can call the resolver directly, outside native YANG validation.

## Consequences

- `ze config validate` needs a static allow-list entry for each new top-level config root, so adding a YANG module is not sufficient to validate it through the CLI.
- OSPF must depend on `interface` as well as `fib-kernel` and `sysctl`, because router-id derivation calls the iface backend during config verification when the operator omits `router-id`.
- The transport now exposes link-up and link-down callbacks because engine running state must track raw socket lifecycle across carrier flaps and config reloads.
- Reload-added interfaces must call `startReceiveLoop` from `openInterface`, not only from initial startup, or a config that starts empty can open sockets without dispatching packets.
- Area IDs need duplicate detection after canonical parsing, since `area 0` and `area 0.0.0.0` are distinct YANG keys but the same OSPF area.
- New plugin packages must be included explicitly in the eventual commit script, because generated `plugin/all` imports can make clean-checkout builds fail if untracked files are omitted.

## Gotchas

- `zt:ip-prefix` accepts IPv4 and IPv6, so OSPFv2 range leaves must use `zt:prefix-ipv4` and still reject IPv6 in the Go parser.
- `0.0.0.0` can pass a naive dotted-quad validator but is the zero RouterID value in Go, so the custom validator must reject unspecified router IDs.
- A slow `HandleLinkUp` can race with `DisableInterface`; recheck `enabled` under the transport lock before publishing the socket or the interface stays joined after removal.
- Functional `.ci` inline block syntax can fail in the parser before semantic validation. Invalid-case fixtures need syntax that reaches the intended verifier.
- Full `make ze-functional-test` currently shows unrelated BMP/RR failures in the plugin suite; OSPF-specific `make ze-ospf-test` is the relevant child proof.

## Files

- `internal/plugins/ospf/register.go`
- `internal/plugins/ospf/config.go`
- `internal/plugins/ospf/area.go`
- `internal/plugins/ospf/instance.go`
- `internal/plugins/ospf/events.go`
- `internal/plugins/ospf/yang/ze-ospf-conf.yang`
- `internal/plugins/ospf/*_test.go`
- `internal/plugins/ospf/transport/transport.go`
- `internal/plugins/ospf/transport/transport_test.go`
- `internal/component/config/validators.go`
- `internal/component/config/validators_register.go`
- `internal/component/config/validators_ospf_test.go`
- `internal/component/config/cli/cmd_validate.go`
- `internal/component/plugin/all/all.go`
- `internal/component/plugin/all/all_test.go`
- `internal/test/cli/register.go`
- `mk/test-functional.mk`
- `Makefile`
- `test/ospf/ospf-config.ci`
- OSPF docs under `docs/guide/`, `docs/architecture/`, `docs/DESIGN.md`, and `docs/functional-tests.md`
