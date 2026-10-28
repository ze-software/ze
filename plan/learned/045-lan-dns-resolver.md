# Learned: a lab that is kinder than production hides the defect it was built to find

`spec-lan-dns-resolver` gave Ze a caching, forwarding DNS resolver for customer
LANs and L2TP subscribers (`service dns-resolver`, plugin
`internal/plugins/dnsresolver`), answering through a second harness wrapper,
`dnsserver.Recursive`, beside `Authoritative`. It is not security-aware and does
not recurse from the root: those are `spec-dns-resolver-dnssec-validation` and
`spec-dns-resolver-recursion`. The design is `docs/architecture/dns/lan-resolver.md`,
the operator view `docs/guide/dns-resolver.md`, and the proof is the interop
scenarios `dns-resolver-forward-unbound`, `dns-resolver-dhcp-lan` and
`dns-resolver-l2tp-subscriber`, each with a recorded red.

## What outlives the spec

1. **A lab setting a real box does not carry hides a product defect.** The L2TP
   scenario's resolver listens on the pool gateway, which exists only while a
   PPP session is up. A lab sysctl (`net.ipv4.ip_nonlocal_bind=1`) made the bind
   succeed, and the scenario went green over a resolver that could never start
   on a real LNS. The test judge refused the sysctl; the fix was the product's:
   `IP_FREEBIND` on the listener sockets (`dnsserver.Options.Freebind`). In the
   other direction, putting a real DHCP client in the lab found Ze's DHCP
   server unicasting an OFFER to an address the client could not yet receive
   on (fixed in dafac43371). Before a lab knob makes a scenario pass, ask whether a deployed box
   has it set.
2. **A test written after its code proves nothing until the code is broken
   under it.** Every interop scenario and every RFC-tagged unit here was
   written against working code, so each was given a red by a recorded break
   (`./le rfc record`, or a reverted producer and a rebuilt image). Several
   first breaks did not build or did not reach the assertion they were meant
   to, and recorded nothing; only the observed red counts.
3. **A review fix is new code and carries the next defect.** Round 1 made the
   cache answer with the held higher-ranked RRset when it refused a
   lower-ranked one (ad5f325dd0). Round 2 found the CNAME chain still followed
   the reply's alias while the answer used the held one, so the reply mixed two
   chains; the fix restarts at the held target (da761bc6c2). Re-review every
   fix with the same care as the first diff.
4. **An agent's report that something is done is a claim to check against the
   committed file.** An earlier phase reported the RFC rows proven, and the
   closer's deliverables check found the resolver-side cases on the RFC 1035
   and RFC 2181 rows the spec required still untagged (cad9658672). Read the
   test file and the record before relaying "done".
5. **An RFC tag's claim names exactly what its assertion observes.** The judge
   refused `TestRFC2181ResolverTTLBounds` as an RFC2181-8-1 positive: its
   assertion treats a high-bit TTL as zero, which is the Section 8 SHOULD
   (8-2), not the MUST (8-1). It refused `TestRFC1035ResolverListensOnServerPort53`
   as a bind proof: its assertion reads the parsed port. Both were retagged or
   narrowed to the assertion (0bf856490c). A claim wider than the body turns an
   unproven requirement into a proven one on the public ledger.
