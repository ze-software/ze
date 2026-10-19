# Learned: one writer guard covers every producer, until a grouping key merges two routes

`spec-bgp-rfc9774-as-set-deprecation` implemented RFC 9774 Section 3: Ze neither
installs nor advertises a route whose AS_PATH or AS4_PATH holds an AS_SET or an
AS_CONFED_SET, with no configuration switch.

## What changed

- Receive: `ASPathSetSegment` (`internal/component/bgp/message/rfc9774.go`) is
  one framed-only scan that both `validateASPath` and `validateAS4PathAttr`
  call, so the two attribute validators cannot disagree. The verdict is RFC 7606
  treat-as-withdraw, which the coalescer, the synthesized withdrawals and the
  diagnostics already handle.
- Send: `Session.writeUpdateBody` asks `transmittedPathHoldsSet` first and
  writes the UPDATE's withdrawal instead (`writeUpdateAsWithdrawal`,
  `internal/component/bgp/reactor/session_rfc9774.go`). Relay, API, static
  routes and plugins all reach that writer, so no list of producers exists.
  Only `send bgp <peer> raw` is exempt (owner, Q-4 (b)).
- `routeGroupKey` now keys on raw attributes, Prefix-SID and the aggregator
  flag.
- Interop `bgp-rfc9774-as-set-frr` judges both halves against FRR 10.3.1 with
  `no bgp reject-as-sets`.

## What outlives the spec

1. **A writer-side guard is only as narrow as the UPDATE it is handed.** The
   static-route builder grouped routes on a key that left out the raw
   attributes, then wrote the first route's attributes for the whole batch.
   So the guard withdrew a clean route along with its AS_SET twin. The guard
   was correct. The key that decided which routes shared an UPDATE was wrong.
2. **Judge every occurrence, not the first.** A configured raw AS_PATH is
   written beside the typed one, so an `AttrFind` that stops at the first
   AS_PATH lets a set through in the second.
3. **When one half of a rule also hides the other, the frames prove nothing.**
   With the receive scan reverted, the send guard still kept the relayed set off
   the peer. The receive half had to be judged on Ze's own RIB and its verdict
   log line.
