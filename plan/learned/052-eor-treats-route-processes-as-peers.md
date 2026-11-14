# Learned: a marker someone asks for is not an automatic one

`spec-eor-treats-route-processes-as-peers` carried the owner's 2026-09-18
ruling (a route-creating process stands where a peer stands, so Ze's
End-of-RIB waits for none) through the code that still acted on the older one.
Thomas then ruled at the research gate that the marker "has no bearing on what
we forward", which withdrew the ordering the spec was opened to build and left
five repairs: a requested marker is never dropped (D-1, `a2302b8968`),
`manual-eor` withholds every automatic marker (D-2, same commit), a recovered
sync panic ends its session (D-3, `2cd7ac9f09`), the session-ready report is
described as what it is (D-4, `4aa1f84ff6`), and the tests assert only the
order Ze promises (D-5, `0461489d41`). `request quiesce` waiting for the
replay fence and the forward queue (`49b01f67ac`) and the FRR scenario
`bgp-manual-eor-requested-frr` (`9609d00703`) were added on the way.

## What outlives the spec

1. **Two kinds of End-of-RIB share one wire format and nothing else.** The
   automatic marker (the sync's, a replay's) is Ze's own claim that its table
   is out, so the claim protocol gives one per family per session. The
   requested marker is the caller's claim, written each time it is asked, as
   ExaBGP does. The defect was one branch in `announceEOR` that judged both by
   the automatic rule, so on a `manual-eor` peer no producer was left. RFC 4724
   holds no "one per family per session" sentence; the comments that cited it
   for that rule were wrong for as long as they existed.
2. **A recovery that clears flags without finishing the work leaves a state
   no one owns.** The sync's panic recovery opened the queueing gate and
   cleared the owed marker over a queue nothing would drain. Ending the session
   is the only recovery whose next state has an owner (the next session's
   sync). Its first version still let the interrupted frame out ahead of the
   Cease: `bufio.Writer.Flush` forgets octets only when the socket write
   returns, so a panic inside it leaves them staged for the next writer
   (`discardInterruptedWrite`). A test judge's refusal of a count-only test
   change found it.
3. **A settle predicate covers every rail that can still write, not only the
   rail its first author knew.** `Peer.pendingSync` read the sync's facts
   alone, so once the marker stopped waiting for processes, `request quiesce`
   settled while a route server's replay was still passing, and a fixture
   stopped the daemon mid-replay. `Peer.syncOwed` reads the replay fence, a
   policy re-send and the forward queue too, and its timeout names the peer
   and the process.
4. **A functional test asking "during the sync" must place its request there
   by construction.** The first `api-eor-during-initial-sync` asked after the
   sync and stayed green against the old drop. Asking from inside the peer-UP
   handler, which the engine awaits before it starts the sync
   (`deliverPeerState`), makes the gate closed on every run; the interop
   scenario uses the same chain. An ExaBGP script cannot be placed there,
   because the bridge acknowledges the event once it has written the line.

## What is left

- A replay-fence owner that never reports makes a quiesce on that peer time
  out, and the forward worker keeps a session after a recovered batch panic:
  both rows are open in `plan/journal/`, owned by `spec-bgp-one-off-defects`.
- AC-7's "the next session sends the queued operations" rests on the existing
  drain of `sendInitialRoutes`; no test drives two sessions through it.
