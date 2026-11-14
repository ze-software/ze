# Learned: the RFC's own sentence decides a ledger row, and a test must reach the code that speaks

`spec-rfc-verdict-fix-services`, child 8 of `spec-rfc-verdict-test-fix-pass`,
took the services group's weak and wrong RFC verdicts (DNS, TFTP, DHCP, flow
export, MCP, config and YANG, ICMP probes) to `enforced`: 85 verdicts over 21
stems on 2026-09-28. 77 ids reached `enforced` (67 from weak, 10 with no
earlier verdict), each with a discrimination record for every tagged test and
a judge that wrote none of them. Nine verdicts remain, each owned by another spec's acceptance criteria:
RFC2131-4.1-7 and 4.1-15 by
`plan/release/fixes/spec-network-services-one-off-defects.md` AC-31 (the
DHCPv4 client's option overload, whose code is missing; the server half is
tagged), RFC7950-7.6.5-1, 7.9.4-1, 8.3.3-1 and 9.1-1 by
`plan/release/fixes/spec-config-yang-when-unique-choice.md` (9.1-1 shared with
`plan/post-release/features/spec-config-yang-reference-types.md`, which states
the split: the release spec owns the accepted forms, the post-release spec the
unsupported ones), RFC7950-9.6.4.2-1 by
`plan/release/fixes/spec-config-yang-loader-structural-checks.md`,
RFC9728-3.3-1 by `plan/release/fixes/spec-mcp-protected-resource-metadata-path.md`
and SFLOW-V5-x-11 by `plan/release/fixes/spec-bmp-sflow-export-rfc-defects.md`.
Four RFC 1035 verdicts of an un-enrolled stem are AC-28 of
`plan/release/validation/spec-dns-existing-support-proof.md`.

The test work exposed three product defects, each fixed failing-test-first:

| Commit | Defect |
|--------|--------|
| `519d9f4f2a` | The TFTP server resent the outstanding DATA on any packet that was not the expected ACK, so a duplicate ACK drew a resend (the Sorcerer's Apprentice bug, RFC 1350 Section 5). `sendAndWaitACK` then retransmitted only on the ACK timeout, and since `c120b09f02` the DATA path waits in `windowSender.awaitACK` under the same rule. `TestTFTPCurlDuplicateACKInterop` proves it against curl: green on the real build, red with a duplicate ACK answered by DATA, green again (`docs/architecture/testing/interop.md`) |
| `9e8443e287` | geodns `checkName` accepted an empty label (RFC 2181 Section 11) |
| `dbdde9fb4f`, `04580da85d` | `parseTunnelRemote` accepted an unspecified remote (0.0.0.0, ::), installing a tunnel that decapsulates from any source (RFC 4213 Section 3.6). The first fix refused it in the netlink backend only; `04580da85d` refuses it at config verify (`TunnelSpec.CheckRemote`) for every tunnel kind and either backend |

The sFlow agent-address advice of Section 4.3 needed no product change: it
already warned, and `12945e76f0` added the tests that prove the warning
reaches the operator.

## What outlives the spec

1. **The RFC's own text decided every ledger item, and the summary's wording
   decided none.** SFLOW-V5-x-15 was retired because its sentence obliges no
   agent; its cumulative-counter meaning is RFC 2233's. RFC1350-5-2 had been
   retired on the ground that Ze refuses WRQ, which an owner rule reverses
   (the TFTP receiver requirements are owed work), so the obligation came
   back as the gap RFC1350-5-5, a fresh ordinal because a retired id stays
   reserved. RFC 2181 Section 10.3 "never a CNAME RR" got no row, because it
   restates RFC2181-10.3-1. RFC1350-2-3's old claim was RFC 1123 Section
   4.2.3.1's. Each call was made by reading `rfc/full/` and quoting the
   sentence, never from the row.
2. **A test that reaches the classifier but not the engine leaves the
   requirement weak.** The sFlow agent-address classifier had unit tests, yet
   the verdict stayed weak: the operator sees the warning only through the
   log call in `runEngine`'s configure closure. The tests that moved it drive
   `runEngine` through the SDK configure callback, with an interface backend
   registered through the production registry, and their records break that
   log call and the classification input.
3. **A naming finding is a question about coverage, not about a marker.**
   Removing SFLOW-V5-x-15's tag left `sflow_v5_register_test.go`, named for
   sFlow v5, with no tag. An `RFC naming: untagged` marker would have been
   false: two of its tests already enforced SFLOW-V5-x-16 for ifSpeed and
   ifDirection, fields the requirement's tagged tests did not cover. Tagging
   them, with an observed red each and a judge's approval, answered the
   finding and widened the proof (`8b6e37aaaa`).
4. **Another spec's producer change stales this spec's records.**
   `537b3a87f9` changed `loadMCPTLSConfig`, so the four RFC9728-7.1-2 records
   no longer verified. They were re-taken with their reds observed
   (`2587658ad3`). A record run that failed while another session cleaned the
   build cache, or while another session's half-applied edit did not compile,
   passed once that work ended: the cause was named before the rerun.
5. **A refusal placed in one backend is a refusal the operator meets after
   commit, and the other backend never makes it.** The unspecified tunnel
   remote was refused in the netlink backend, so `ze config validate` and a
   commit accepted it, and so did the VPP backend. Config verify is the one place
   every backend passes through (`04580da85d`). The closure review found it
   because it read the producer of a fix the spec had recorded as done,
   rather than the spec's account of it.
