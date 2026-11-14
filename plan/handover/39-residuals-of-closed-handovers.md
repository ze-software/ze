# Residuals of closed handovers

This page keeps the few items from closed handovers that nothing else in the tree records.
Each row is removed when its item gets an owner (a spec or a journal row) or an answer.

## Decisions for Thomas

| Question | Current state | What each answer changes | Source |
|----------|---------------|--------------------------|--------|
| Should `update hex` and `update b64` adopt the `nlri` / `mp <family>` section keywords that `update text` now uses? | `parseWireNLRISection` (`internal/component/bgp/plugins/cmd/update/update_wire.go`) still parses `nlri <family> [addpath] add <data>`; `docs/guide/route-injection.md`, "Hex Format", shows `nlri ipv4/unicast add 180a0000` | Yes: the wire parser takes `nlri` for IPv4 unicast and `mp <family>` for the rest, refusing the old form by name as `update text` does, and the docs and tests that use the old form are converted. No: the two grammars stay different and the page says so | 02-session-2026-10-17-tests-green-and-rfc |
| Should the config form `update { nlri { <family> add ... } }` follow the same grammar? | Unchanged by the `update text` grammar change; still takes a family under `nlri` | Yes: the YANG and its parser change, and every config in the repository is converted directly (no migrate path, pre-release rule). No: config keeps the family form | 02-session-2026-10-17-tests-green-and-rfc |
| Should the received-event text (`received update ... nlri <fam> add`) follow the same grammar? | Unchanged; only command lines changed | Yes: event consumers (plugins, the ExaBGP bridge, text-format docs) change with it. No: events keep naming the family under `nlri` | 02-session-2026-10-17-tests-green-and-rfc |
| When `next-hop self` resolves to an IPv6 address for an `nlri add` route, should the reactor move the route into MP_REACH_NLRI? | Recorded as possible on 2026-10-17; not decided | Yes: an IPv4 route with a self next hop that is IPv6 is sent in MP_REACH_NLRI (RFC 8950 form). No: the reactor's handling stays as it is today | 02-session-2026-10-17-tests-green-and-rfc |
| Should a hook refuse shell writes (sed -i, python, redirects) anywhere in the repository, not only in `plan/` and `ai/rules/`? | `bashGovernedWrite` (`internal/le/hookruntime/bash.go`) refuses a shell write into `plan/` or `ai/rules/` only; agents had bypassed the edit hooks with python and sed elsewhere | Yes: every tracked file is edited only through Edit or Write, so the post-edit formatting and the write hooks see every change. No: shell writes stay allowed outside the two governed trees | 28-config-list-order-phase-2 |

## Runs owed on Linux or Docker

| Command | What it proves | Source |
|---------|----------------|--------|
| `./le test qemu all` (the runner the `test/plugin/show-mtu-*.ci` headers name; they need root for CAP_NET_ADMIN and CAP_NET_RAW) | The six `show-mtu-*` tests pass after 892b01fa35 (show mtu measures against the configured `ze.mtu.reference-address`, else the first name-server, else refuses) | 02-session-2026-10-17-tests-green-and-rfc |
| `IPSEC_INTEROP_SCENARIO=mtu-nat-installed-endpoint ./le test integration interop-ipsec` | The scenario passes after 892b01fa35; its last recorded run (nightly 39015310430) was red on a docker exec timeout | 02-session-2026-10-17-tests-green-and-rfc |
| `./le --name session test bgp plugin --pattern pki-certificate-export-show`, then the same with `as112-dot-pki` and `geodns-dot-pki` | The three `test/plugin/` tests pass on Linux after the refuse-by-default work | 24-refuse-by-default-implemented-awaiting-closure |
| `./le test unit pkg path ./internal/component/host/` | The thermal tests (`thermal_linux_test.go`) pass on Linux | 24-refuse-by-default-implemented-awaiting-closure |
| `./le test unit pkg path ./internal/plugins/trafficusage/ run TestTCXProbeRuntimeReadiness`, once as a normal user and once as root (inside the QEMU guest) | `probeTCX` answers ready, absent or unknown from the kernel in both privilege cases | 24-refuse-by-default-implemented-awaiting-closure |
| `./le test unit pkg path ./internal/component/l2tp/` | The l2tp `*_linux_test.go` tests pass on Linux | 24-refuse-by-default-implemented-awaiting-closure |
| `./le test unit pkg path ./internal/plugins/iface/dhcp/ run TestDHCPv6AddressesWithoutDADOutcomeNotReadAsUnique` | Green as committed, and red with the `DADReported` block in `checkDAD` deleted (the red walk) | 24-refuse-by-default-implemented-awaiting-closure |
| `./le test unit pkg path ./internal/plugins/iface/netlink/ run TestNetlinkGetStats` | netlink `GetStats` returns the read error and never zeroed counters for a link with none | 24-refuse-by-default-implemented-awaiting-closure |
| `INTEROP_SCENARIO=addpath-zero-id-frr ./le test integration interop` on the Ze-kernel Docker host | FRR accepts an ADD-PATH route with path id 0. This proves acceptance only; its red phase needs `nlri.WriteNLRI` mutated to drop the identifier octets for id 0 | 24-refuse-by-default-implemented-awaiting-closure |
| `INTEROP_SCENARIO=bgp-addpath-best-path-pmacct ./le test integration interop` on the Ze-kernel Docker host | pmacct still decodes the BMP Loc-RIB messages, whose wire form the refuse-by-default work changed | 24-refuse-by-default-implemented-awaiting-closure |
| `go test -tags docker ./internal/component/gnmi/` | gnmic with its default encoding works against Ze's gNMI server, which now serves JSON and JSON_IETF and answers `Unimplemented` for other encodings | 24-refuse-by-default-implemented-awaiting-closure |

## Small work with no owner

| Item | Place in the tree | Size | Source |
|------|-------------------|------|--------|
| Classify each violation `./le rfc check` reports: route each to its owning spec or a journal row. 76 were reported on 2026-11-06 and never classified; `plan/release/validation/spec-rfc-figures-built-and-proven.md` does not mention them | `./le rfc check` (`internal/le/rfc`) | One run and a triage pass | ci-41685742426-history-followthrough |
| Boot record fsync trim 3: drop the fsync cost of `PromoteCandidate`'s `file/active/<name>` mirror. Judged not safe: bare-name reads resolve to the mirror (`resolveKey`, `store.go`) and `ReadRestoreSource` reads it when no pointer exists, so every ReadKey or ReadFile of a bare config name needs an audit first. Trims 1 and 2 have landed | `PromoteCandidate` (`internal/component/config/storage/pointer.go`), `resolveKey` (`store.go`), `ReadRestoreSource` (`restore.go`) | An audit of bare-name reads, then a small change | 02-session-2026-10-17-tests-green-and-rfc |
| A QEMU guest that stops while it holds the `le` build lock leaves it for 30 minutes, so a host commit made after a guest run waits that long for its `le` build. The lock records the guest's host id, and the host judges another host's lock busy until it is 30 minutes old | `lock_held` in `./le` (the launcher script, the `lock=$builds/.building` block) | Small: the guest releases its lock before it powers off, or the host can tell a guest that has exited | 2026-11-07-home-cpe-implementation |
| The spec-edit gate counts any Read of a producer as reading it, however short: one returned line satisfies it. A reported 3-line read passed | `currentSourceReads` and `sourceRead.validRange` (`internal/le/hookruntime/designevidence.go`) | Small: decide what read proves a producer was read (the whole file, or the cited function's range) and test it | 2026-11-11-agent-economy |
