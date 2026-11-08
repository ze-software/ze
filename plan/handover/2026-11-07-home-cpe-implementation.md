# Handover: home-CPE umbrella implementation (2026-11-07 to 2026-11-08)

Umbrella: `plan/release/fixes/spec-home-cpe-0-umbrella.md`. Goal set by Thomas:
complete the home-CPE implementation; all testing in the main thread, one
CPU-heavy job at a time; agents develop in parallel while tests run.

## How to run things (learned the hard way)

- Go tests: `tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"`, run through `./le job run label <x> command go test -tags "$tags" ...`. Linux-only tests add the `integration` tag and run in QEMU.
- QEMU: `./le test qemu run kernel /Users/thomas/.cache/ze/runtime-kernel/7.3-runtime-arm64-runtime-docker-a9caebdd-0d1d6496/vmlinuz packages "nftables iproute2 libcap git" timeout 55m command "sh <script>"` with the sandbox off. The guest has no bash (use sh). Scripts used: `tmp/session/2026-11-07-104251/scratch/lx*.sh`.
- `.ci` fixtures and interop labs run the `le` built from HEAD, never the working tree: commit fixture and lab code before a `.ci` or scenario can use it, then wait for the new build (`ls bin/builds/*-<sha>`; a `.failed-<sha>.log` names a startup refusal).
- The guest builds its own linux-arm64 `le` from HEAD in the background; the guest script waits for `bin/builds/*-<HEAD>-linux-arm64`.
- `./le test qemu netns suites pppoe` and `./le test qemu all test <path>` run only inside the guest, and need host-built `bin/ze-linux-arm64` and `bin/ze-stripped` first (not done yet: find the le build step that produces them).
- IPsec interop selects one scenario with `IPSEC_INTEROP_SCENARIO=<name>`; PPPoE with `./le test deployment docker-pppoe-accel` (`ZE_PPPOE_INTEROP_SCENARIO=<name>`); topology with `./le test integration interop-topology [scenario <name>]`.
- Never run a plain `go mod vendor`: it drops the vendor patches. Use `./le setup install`.
- The Ze Docker host is the Colima `ze` profile (`./le setup docker host start`).

## Proven today (Linux or interop, committed)

| Child | Evidence |
|-------|----------|
| 1 | static follows-device .ci; named-device reconnect against accel-ppp; PPPoE session socket binds the resolved device (TestDialOSNameSourceInterface on Ze 7.3); connected sees addresses that predate the monitor (red walk recorded) |
| 2 | five dhcpserver .ci promoted and green on Ze 7.3 |
| 3 | three conntrack kernel tests green; conntrack .ci trio green |
| 4 | fqdn-remote-dynamic-local and fqdn-remote-real-nat pass against strongSwan |
| 5 | Ed25519 and Ed448 both directions pass against strongSwan (circl v1.6.5, owner-approved, ledger row) |
| 6, 20 | vti-ipv6-inner-ipv4-outer passes (after the xfrm-interface catch-all fix and the MODP_2048 rekey fix); child 20 AC-1 red walk recorded |
| 7 | appliance-hostname QEMU proof; V6ONLY L2TP listener test |
| 8 | IPv6CP scenarios pass against accel-ppp; DHCPv6 identity device resolves through the logical name |
| 9, 21 | clamp tests and policy-tcp-mss-clamp.ci green; child 21 AC-1 red walk recorded |
| 13 | DNS kept in the session result; DNS scenarios pass (PPPoE suite 14/16, the 2 reds are Ze-as-AC IPv6CP owned by post-release spec-l2tp-ipv6-subscriber) |
| 14 | realm relay Linux tests 5/5; relay-realm-config validates offline |
| 22 | responder-reauths-ze-initiator and the port-translated NAT reauth pass |
| IPsec suite | 71/72 then the last red fixed: full suite green apart from what later runs show |

## Open work

| Item | What is left |
|------|--------------|
| Topology lab (11, 12, 14, 15, 18, 19) | rerun in progress at handover (`topo-full4.log`); xl2tpd LNS and DHCPv6-PD default route need the new log-file diagnostics read |
| Netns runs | child 12 `pppoe-client-lcp-renegotiation`, child 14 `pppoe-realm-relay-nomatch` (both in test/pppoe and the netns list), child 22 `ipsec-respond-reinit-supersede` and child 4 `ipsec-follows-wan-address` drafts: need host-built ze binaries, then run in the guest |
| Child 8 phases 4-5 | groundwork committed (config, RA RIO, lease lifetimes); LAN consumer, RA Update, Release hold (RFC 9915 6.3-3), interop: design in the session state file `tmp/session/2026-11-07-104251/state/session-state-home-cpe-8-...` |
| Child 21 | AC-2 red walk (four-byte write) |
| Child 3 | red walks and RFC 4787 REQ-10 records |
| Child 4 | RFC 7296 2.23 records |
| Child 10 | review round 2 (round 1 findings fixed) then closure |
| Child 7 | listener-migration-ssh draft run and promote |
| Closures | children 1, 2, 5, 6, 7, 10, 13, 17, 20, 22 once their remaining items land (ze-close; all five checked children are still `design`, and the WIP cap is full: close before claiming) |
| SPD selection spec | phases 2 and 4 written, uncommitted (engine interface catch-all, internal/core/iprule move, policyroute uses it; 4 old netlinkint files to `remove`); phases 3, 5, 6, 7 left; guard table range collides with operator tables 3000-3999 (fix: refuse from iprule.ZeTableRanges) |
| Child 23 DNS | design; decisions recorded: O-1 explicit fall-through mode, default pass-through, O-6 own spec after the umbrella, RFC 5625 inside child 23; open: O-3, O-4, O-5, O-8, O-9 |
| New specs | ze doctor audit of every operator-reachable problem (being written), RFC 8420 enrolment as part of the vpn-concentrator work (being written), IPv6 subscriber DNS (after the umbrella) |

## Owner decisions taken today (all recorded where they belong)

- ai/META-DECISIONS.md: a third-party module is approved only for its approved use; RFC 9463 DNR not in the first release; ze doctor reports every problem an operator can hit.
- SPD selection: the complete multi-SPD design, implement it.
- RFC 9915 6.3 rows are requirements (the "2026-10-19 decline" had no decision behind it).
- circl approved for Ed448 in IKE only; the dependency ledger and `./le deps approve` gate built.
- Taken by default, not overruled: child 7 SSH boot policy, child 3 all ten helpers built in, child 17 keepalive counts N lost echoes.

## Known reds that are not this work

- Five internal/le/commit tests since 3475223c4b and 8c64ad2d4a (fixture specs without a metadata table; attestation script text).
- TestEveryIntegrationPackageIsNamed names internal/le/site.
- Two MOBIKE tests need a 127.0.0.2 loopback alias on macOS.
