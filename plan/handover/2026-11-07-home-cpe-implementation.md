# Handover: home-CPE umbrella implementation (2026-11-07 to 2026-11-08)

Umbrella: `spec-home-cpe-0-umbrella` (closed). Goal set by Thomas:
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
| Topology lab (11, 12, 14, 15, 18, 19) | done: every scenario green against current builds (access-up and lan-services after lab fixes; pppoe-dhcpv6-pd-ze-client after the PPPoE client opened IPv6CP beside IPCP and the checker learned Rapid Commit) |
| Netns runs | done: netns PPPoE 10/10 on Ze 7.3; child 22 `ipsec-respond-reinit-supersede` (6/6) and child 4 `ipsec-follows-wan-address` (3/3) promoted after running on Ze's own image through the docker-lab harness |
| Child 8 phases 4-5 | interop green end to end (LAN host default route via Ze, traffic, bounded RA lifetimes, Router Lifetime 0 after terminate); `show interface delegated-prefix` and pppoe-client `delegated-prefixes` committed; the two interop red walks running (GOFLAGS overlays in scratch/rw8). Spec rows (Interop row "Phases 1 to 3 only", AC-6 gap) still to update at closure |
| Child 21 | AC-2 red walk (four-byte write): QEMU script ready at scratch/redwalk/home-cpe-21-ac2.sh, not yet run |
| Child 3 | done: RFC4787-7-1 and 7-2 tagged and recorded on Ze 7.3. The five manual red walks: scratch/redwalk/home-cpe-3.sh, not yet run |
| Child 4 | done: five RFC 7296 2.23 records. Spec text still says the bus is the address source (lines ~114, 123, 222): now the kernel address-table watch too |
| Child 10 | review round 3: one ISSUE (row classes) fixed and committed; a round 4 limited to rows 131-132 is owed, then closure |
| CERTREQ | unit red walk script ready (scratch/redwalk/certreq-unit.sh); lab red walk via GOFLAGS overlay on mgmt-tunnel; no RFC row (3.7 is a MAY) |
| Defects found by running on Ze's own image, all fixed with red walks | ESP socket opened by name (no /etc/protocols on gokrazy); route watcher setns needing CAP_SYS_ADMIN (blinded the RA sender, fib-kernel monitor, kernel redistribution); IPv6 forwarding needs the global switch; IKE dynamic WAN only followed the bus (no interface block, no monitor); deleted WAN interface kept peers; netns runner copies on a nosuid tmpfs; docker-lab cache symlink |
| Tooling friction | every QEMU guest leaves a bin/builds lock the host only reclaims after 30 minutes, so a commit made after a guest run waits that long for its le build |
| Child 7 | done: listener-migration-ssh promoted (QEMU, host, 60 stress runs) |
| Closures | children 1, 2, 5, 6, 7, 10, 13, 17, 20, 22 once their remaining items land (ze-close; all five checked children are still `design`, and the WIP cap is full: close before claiming) |
| SPD selection spec | phases 1 to 7 committed and green (units, Ze 7.3 kernel tests, three strongSwan scenarios with red walks, RFC4301-4.4.1-5 records). Interop found and fixed: the guard routed only the first selector pair, its drop caught the packet entering the transform, route events and rekeys carried only the first pair. Left: the spec's red-walk row for the leak scenario (the guard alone cannot leak; the transform-term removal is the walk that was run), then closure |
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
