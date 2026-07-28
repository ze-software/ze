# bgp-attribute-order-one-convention

Every builder that emits a BGP UPDATE now writes path attributes in ONE order:
MP_UNREACH (15) first, then everything else -- MP_REACH (14) included -- by
ascending type code.

## Why this kept biting

RFC 4271 Section 5 leaves attribute order free ("A BGP speaker MUST be prepared to
accept attributes in any order"), so no peer ever complains and no decoder ever
fails. The damage is entirely internal: an exact-hex functional test stops being a
test of the ROUTE and becomes a test of the RAIL, decided by scheduling.

This was the third round. `reactor/peer_rib_routes.go` already carries the scar
tissue from two earlier ones (the batch builder appending LOCAL_PREF and MP_REACH
after EXTENDED_COMMUNITIES; then that builder appending AS4_PATH after
LARGE_COMMUNITIES). Both were fixed by moving the attribute to its type-code
position. What nobody checked was the third builder.

## The three builders, and the one that disagreed

| Builder | Order | Used by |
|---------|-------|---------|
| `reactor/peer_rib_routes.go` buildRIBRouteUpdate | type code | initial-sync drain |
| `reactor/reactor_api_batch.go` buildWireModeUpdate | type code (insertAttrOrdered) | post-establishment sends |
| `attribute/origin.go` OrderAttributes | **MP_REACH last** | `message` package builders, config/static routes |

`peer_rib_routes.go`'s own ordering note CLAIMED the third sorted by type code. It
did not, and had not since it was written. A comment asserting another file's
behaviour is worth exactly as much as the last time someone checked it.

## How it surfaced

`test/plugin/flowspec.ci` asserted EXT_COMMUNITIES before MP_REACH, because a
static config route (OrderAttributes) produced its bytes. The same route announced
through `update text` produced the reverse. The test only noticed once the route
actually started coming from the API path -- until then a static route in the same
config had been silently satisfying the assertion.

`writeRelayPayload`, fixed the same day
(`plan/learned/1283-fixit-ci-plugin-suite-nine.md` Shape 2), was a FOURTH instance
in a different shape: it appended a re-synthesized MP_REACH rather than slotting it
where the source had it.

## Measure before choosing a direction

The first instinct was "one builder is the outlier, fix it". Measuring said
otherwise: MP_REACH-last was pinned by **15 of 55** `test/encode` expectations,
i.e. it was the established order for a whole family of tests, not a stray. That
changed the question from "fix a bug" to "pick a convention and pay for it", which
is an owner decision rather than a cleanup.

Final cost, once chosen: 18 `.ci` files (52 expectations) and 3 Go fixtures.

**Do this before touching a shared ordering function.** Apply the change, run the
suites, count. The number decides whether it is a fix or a migration.

## Rewrite expectations by TRANSFORM, never by paste

The 52 expectations were rewritten by a script that parses the COMMITTED hex,
reorders the attribute block, and re-emits -- asserting per rewrite that the
attribute set, every attribute's bytes, and the total length are unchanged.
Anything failing those assertions is left alone and reported.

Pasting the daemon's output would have produced the same green suite while
encoding whatever else had changed at the same time. The transform cannot: it can
only permute what was already there, so a green suite afterwards means the bytes
moved and nothing else did.

## Related

- `plan/learned/1283-fixit-ci-plugin-suite-nine.md` -- Shape 2, the relay-rail
  instance of the same defect, and the sweep this came out of
- `internal/core/bgp/attribute/origin.go` -- OrderAttributes, and why MP_UNREACH
  stays first, out of type-code order
