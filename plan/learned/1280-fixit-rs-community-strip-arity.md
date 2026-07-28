# 1280 -- fixit-rs-community-strip-arity

## Context

A route-server client received the route server's OWN control communities -- the
`0:<asn>` and `<rs-asn>:<asn>` tags used to steer per-peer forwarding -- whenever a
route carried **two or more** of them. With exactly one, the strip worked. The leak
had no test and no user-facing documentation, so nothing could notice it.

The two halves of the modification-accumulator contract disagreed about arity, and
neither half was wrong on its own.

`StripControlCommunities` (`internal/component/bgp/wireu/community.go:158`) walks the
COMMUNITY attribute and accumulates EVERY matching four-byte value into one slice.
Both route-server rails hand that whole slice to the accumulator as a single Remove
operation (`reactor/reactor_api_forward.go:642`, `reactor/forward_rs.go:347`).

The consumer, `removeValues` (`filter_community/handler.go`), opened with
`if len(toRemove) != valueSize { return data }`, carrying the comment
`// Size mismatch: caller bug, silently preserve data`. Two communities is eight
bytes against a value width of four, so the guard tripped and removed NOTHING.

Two defects, one site. The leak, and a guard that had already diagnosed the fault and
then declined to report it (`ai/rules/fail-closed-guards.md`).

Every other producer already split into per-value operations -- the text-delta path at
`reactor/filter_delta.go:221-224`, the community plugin's own egress filter at
`filter_community/egress.go:28-30` -- so the one-value rule was a real contract that
had simply never been written down, and the two route-server sites were its only
violators.

## Decisions

- **Widen the consumer, do not split at the producers.** Splitting fixes the leak and
  leaves the fail-open guard in place, so the next producer to violate the rule fails
  silently in exactly the same way. `removeValues` now treats its buffer as a SET of
  whole values; a length that is not a whole multiple is refused. One value is a whole
  multiple, so every existing single-value producer is untouched.
- **Return the refusal as a value, do not log it from the helper.** The spec called for
  the log line inside `removeValues`. `logger` in this package is
  `slogutil.LazyLogger("bgp.filter.community")` (`filter_community.go:26`), memoised
  behind a `sync.Once`, so a helper that logged directly could only be tested through
  logging configuration. `removeValues` returns `([]byte, bool)`; `genericCommunityHandler`
  emits the warning and `continue`s. That also satisfies "the attribute's remaining
  operations still apply" -- one producer's bug must not become a second, wider
  behavior change by discarding its well-formed siblings.
- **Document the obligation on `ModAccumulator.Op`** (`filterapi/filterapi.go`), where a
  filter author reads it, not only on the handler. `Op` does NOT validate: it has no
  attribute-width table and runs per forwarded UPDATE, so the check belongs at the
  handler that already knows its own value width.
- **No refusal counter.** The spec listed one as a design decision, but it is not an AC,
  and this package has no metrics surface at all. Introducing one for a path that should
  never fire was left as an owner decision; the warning names the attribute code, the
  expected width and the actual length, which is enough to identify the producer from a
  single line.

## Consequences

- A route carrying any number of control communities now has all of them stripped, on
  both rails. Ordinary communities survive in their original order.
- The fix is width-independent: COMMUNITY (4), EXTENDED_COMMUNITY (8) and
  LARGE_COMMUNITY (12) share `genericCommunityHandler` with only `valueSize` differing.
- First coverage of this path: two `.ci` files, one per rail, plus unit tests on both
  the producer and the consumer. `StripControlCommunities` previously had four
  references in the tree and not one was a test.
- First user-facing documentation of the convention:
  `docs/guide/bgp-policy.md` "Route-server control communities".
- Behavior change for operators who had (unknowingly) come to rely on seeing the leaked
  tags. It contradicts the stated intent at `wireu/community.go:138-140`, so the leak is
  the bug, not the behaviour being removed.

## Gotchas

- **`session/rs-client true` is mandatory in any `.ci` covering this path, and its
  absence is invisible.** An early draft of both tests omitted it. The route forwarded,
  the test ran, and all five communities arrived intact -- byte-for-byte the symptom of
  the bug the test was written to catch, against a FIXED binary. The leaf defaults to
  false (`reactor/config.go:266`) and gates both the RFC 7947 policy block
  (`reactor_api_forward.go:611`) and the strip emission (`:642`). A `.ci` without it
  proves nothing while looking like it proves everything. The tell is AS_PATH: with
  `rs-client true` the route server does not prepend its own AS (RFC 7947 2.2.2), so
  `AS_PATH [65001]` rather than `[65000 65001]` confirms the leaf took effect.
- **The functional-test runner resolves the DUT from `tmp/s/<session-id>/bin` BEFORE
  `bin/`** (`internal/test/sessionpath/sessionpath.go:107-132`, `FindPrebuiltDir`). With
  `ZE_TEST_NO_BUILD=1`, a stale session-scoped binary is used silently: rebuilds of
  `bin/ze` have no effect and debug probes never fire. This cost an hour and sent the
  investigation looking for a third forwarding rail that does not exist
  (`ForwardUpdatesDirect` delegates to `forwardUpdateCore` at
  `reactor_api_forward_batch.go:148`). Run WITHOUT `ZE_TEST_NO_BUILD=1` and let the
  runner build the DUT.
- **A bare `go test ./internal/component/bgp/...` fabricates reds.** Without the feature
  tags from `feature-gates.txt` it reports `unsupported family` failures in `bgp/cli`
  and `bgp/config` that have nothing to do with the change under test.
- **Whitelist beats blacklist.** `ShouldForwardTo` (`wireu/community.go:21-32`) returns
  early on the whitelist, so a single `<rs-asn>:<asn>` tag makes every `0:<asn>` tag on
  the same route inert. When choosing control communities for a test, a `0:<asn>` naming
  the receiver suppresses the forward entirely and the test then proves nothing about
  stripping.
- The selection rule matches on the community's HIGH HALF alone, so an ordinary
  community whose high half happens to be 0 or the route server's low sixteen ASN bits
  is stripped too. Pre-existing and unchanged -- but the fix makes stripping actually
  happen, so the ambiguity is reachable for the first time in the multi-value case.
- Large-community control values are PARSED (`parseLargeCommunityAttr`) but never
  STRIPPED: `StripControlCommunities` only inspects code 8. Separate defect, out of
  scope here.

## Files

- `internal/component/bgp/plugins/filter_community/handler.go` -- `removeValues` returns
  `([]byte, bool)` and accepts a whole number of values; new `containsValue`;
  `genericCommunityHandler` warns and continues on a refusal.
- `internal/component/bgp/plugins/filter_community/handler_test.go` -- multi-value,
  single-value-unchanged, loud-refusal, all-widths, empty-attribute cases.
- `internal/component/bgp/wireu/community.go` -- doc comment states the concatenated
  multi-value return, the 16-bit high-half limit, and that stripping is ze's own
  behaviour rather than an RFC 7947 obligation.
- `internal/component/bgp/wireu/community_test.go` -- NEW. Producer-side coverage plus
  malformed-payload bounds cases.
- `internal/component/bgp/filterapi/filterapi.go` -- `ModAccumulator.Op` documents the
  list-valued Remove arity obligation.
- `internal/component/bgp/reactor/reactor_api_forward.go`,
  `internal/component/bgp/reactor/forward_rs.go` -- comments naming the contract each
  emission site relies on.
- `test/plugin/bgp-rs-community-strip-multi.ci`,
  `test/plugin/bgp-rs-community-strip-multi-fastpath.ci` -- NEW, one per rail.
- `docs/guide/bgp-policy.md` -- new "Route-server control communities" section.
