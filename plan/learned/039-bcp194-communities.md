# Learned: a route server acts on the controls it strips, under one ASN

`spec-bcp194-1-communities` finished the BCP 194 community mechanisms Ze ships
for a route server: the large `RS:101..103:<client>` prepend request
(RFC 7947 Section 2.2.2.1, RFC 8195 Section 4.2.1) applied on both forward
rails, every large control value in the route server's namespace stripped as
one code-32 `AttrModRemove`, the RFC 7454 Section 11 own-GA scrub keeping a
route-server client's control values, the RFC 2119 key-words sentence kept
out of the RFC ledger's obligation counts, and five Prometheus counters. The
reference pages are `docs/guide/bgp-policy.md` ("Route-server control
communities", "Ingress community rules"), `docs/architecture/core-design.md`
(the strip contract), `docs/architecture/meta/filter-community.md` and
`docs/plugin-development/metrics.md`.

## What outlives the spec

1. **The set the scrub keeps is the set the route server acts on and
   strips.** The forward rails read the cached, post-filter payload, so an
   ingress scrub that removes a control value silently disables the request.
   The keep matches through the same predicates the rails use
   (`wireu.IsRSControlCommunity`, `wireu.IsRSControlLargeCommunity`) and
   against the same ASN: the global local-as, carried on the received path as
   `filterapi.PeerFilterInfo.RouteServerAS`. A per-peer local-as override is
   never the route server's ASN; matching on it leaked override-shaped
   controls to every client.
2. **Both forward rails make every RS edit, or the edit leaks on whichever
   rail a deployment selects.** `reactorForwardRSSection` and
   `forwardUpdateSection` carry the identical strip and prepend code, and each
   test and each counter assertion runs on both.
3. **A counter's unit is stated once and holds on every path.** The RS
   counters first counted once per forward call, which made a received UPDATE
   split across calls (AIGP mixed fields, the withhold redo) count per section
   while an in-call split counted once. They now count each UPDATE queued to a
   client, the place the edit takes effect, and the help text, the code
   comment and the page say so.
4. **A protocol change owes an interop scenario even when every unit and
   `.ci` test is green.** Closure added `bgp-route-server-control-community-frr`
   (FRR client, Ze route server, BIRD client); its reds were observed under an
   `RSPrepend` no-op and a nil large strip.
