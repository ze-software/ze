# Handover: home-CPE umbrella implementation (2026-11-07)

Umbrella: `plan/release/fixes/spec-home-cpe-0-umbrella.md`. Goal set by Thomas:
complete the home-cpe implementation, all testing in the main thread, order
chosen by the session (the umbrella's executable order, with independent
children run in parallel).

## What landed today (commits on main, not pushed)

| Child | Commits | State |
|-------|---------|-------|
| 1 logical interface name | 1d7b88a626, a65e5c69c6, a8932e4e13, 71fac007f6, 5982501268, 69ecaa5a44 | implementation complete; Linux tests, static drafts, parse drafts and the `pppoe-named-device-reconnect-ze-client` scenario not yet run |
| 2 DHCP shared subnets | 10e9995d31 | phases 1-3 done; phases 4 (server identifier), 5 (interop), 6 (docs) owed; no DHCP probe client exists for `.ci` |
| 3 conntrack helpers | fb65e64acf | done in the firewall transaction; integration netns tests not written; drafts unrun; kernel rebuild unproven |
| 4 IKE dynamic WAN | 67700fd37e | most ACs; interop `fqdn-remote-*` need a DNS harness in the IPsec lab; RFC 7296 2.23 tags not recorded |
| 6 IPv6 in IPv4 VTI | e8faddced1 | done; scenario and red walk unrun; parse draft unrun |
| 7 system small defects | 7ff7614e5f | done; QEMU `appliance-hostname` unrun; drafts unrun |
| 8 IPv6 CPE chain | 69551c0e7f (ppp codec in 79e3205310) | phases 1-3; phases 4-5 wait on owner (RFC 9915 6.3 decline) |
| 9 clamp to PMTU | ba972d0bb6 | done; Linux tests, draft, scenario unrun; site golden may need judged refresh |
| 10 VyOS guide | a83df302de | done; parse draft unrun (skips darwin) |
| 13 IPCP DNS | 518401eb75 | done except LAN resolver use (owner decision) and interop runs |
| 14 LAC realm relay | 79e3205310 | partial: bridged-session teardown, connecting-tunnel queue, interop scenarios, `.ci` drafts owed |
| 17 keepalive config | db2ea98439 | done; Linux end-to-end and parse draft unrun |
| 18 interop topology | 60c615ead1, f765f6e9fd (help fix) | done except AC-7b (RSVP-TE VLAN to freeRouter rawInt needs an external-end link form) |
| 19 access chain | 93362da68e | written; needs Docker host on Ze's kernel |
| 20 XFRM AF_UNSPEC | 0db8414cfd | done; Linux kernel tests and red walk unrun |
| 21 MSS clamp proof | f19ada1079 | done; netns tests unrun |
| 22 IKE responder reauth | 67700fd37e | core only; CREATE_CHILD_SA/DPD/Delete on a held successor, simultaneous replacement, interop owed |
| 12 | 86bc403a4e (earlier) | code landed; closure waits on 14 and 18 scenario `access-wholesale-accel-lns` |

Defects fixed on the way: crashlog test setup raced process stderr (8fdc144540);
stale web L2TP golden (3817635b77); L2TP sent a zero-length datagram whenever the
send window was full (79e3205310); IKE rebind opened before closing (67700fd37e).

## Not started

- Child 15 (LNS proxy-lcp renegotiate): an agent was implementing it at handover; check `git status internal/component/l2tp` for its uncommitted work.
- Child 5 (EdDSA): spec in design, registration redesign awaits Thomas.
- Child 11 (the lab): composes every other child's scenarios.

## Uncommitted, owed

- `plan/journal/unwired-feature.md`: the 2026-09-04 `remote-address` row rewrite (line 66) and the new 2026-11-07 IKE-over-IPv6 row (line 67) are trimmed but uncommitted: the IPv6 row has no owner. Add it to `spec-home-cpe-4-ike-dynamic-wan.md`'s Owned journal rows (or `spec-yang-single-source-of-truth.md` line 215, which still lists the pre-split row); both spec gates need their doors read first. Specs 6 and 20 still cite "the 2026-09-04 row, second finding".
- `plan/journal/README` cap is 600 characters per row.

## Blocking for Thomas

| Decision | Why |
|----------|-----|
| Allow the QEMU guest outside the shell sandbox (`./le test qemu run` fails: "clean child process group before reap: operation not permitted"; the unsandboxed retry was refused by the permission classifier) | Every Linux-only proof above (netns, kernel, parse drafts that skip darwin, appliance boot) is blocked |
| Docker host on Ze's kernel (`spec-docker-hosts-run-the-ze-kernel`) | Every interop/topology scenario |
| Child 7: an extra SSH listener failing at boot keeps today's policy (no SSH, one warning) | taken by default, pending you |
| Child 3 D-1: all ten accepted helpers built into Ze's kernel | taken under your Option A |
| Child 8 phases 4-5: lift RFC 9915 6.3 decline | your scope call |
| Child 13: LAN dns-resolver forwards to IPCP-learned servers? | your call |
| Child 5: EdDSA registration redesign | awaiting approval |
| Child 18 AC-7b: external-end link form vs narrowing the AC | proposed: build the external-end form |

## Process notes

- Run Go tests with the feature tags: `tags="ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')"`; an untagged run gives phantom reds.
- Known environmental reds on this Mac: MOBIKE tests needing 127.0.0.2, `cmd/ze/hub TestStreamingOneShotAnswersCarryData` (raw ICMP), qemu/deployment tests the sandbox breaks, `TestEveryIntegrationPackageIsNamed` (internal/le/site, internal/test/pppoeac).
- Agents that edit RFC-tagged tests must get an independent judge first; the judge may pre-approve narrowed wording and apply it.
- Build commit file lists from explicit paths, not whole directories: 79e3205310 swept in child 8's untracked files.
