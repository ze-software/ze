# A rule is stated as an outcome a later layer decides

Two layers sit on one path. The first one decides what is DISPATCHED, the second
decides what reaches the wire. The author of the first layer knows the one case
in front of them, so they write its rule down in terms of the end result: "this
puts nothing on the wire". The sentence is true for that case and false for
every case where the second layer answers differently, and nothing goes red,
because no gate reads a page and no test asserts a sentence.

The tell is a rule table whose "what it does" column names a WIRE outcome while
the code under it names a DISPATCH. Ask which layer owns each word in the cell.
A rule states what its own layer does; where the reader needs the end result,
the page names the second layer and links to it.

The same shape appears in a design that is CORRECT: this repository met it as a
temporal distinction two layers apart. Two bursts both withdraw a route the peer
never held, and only the second reaches the wire, so no rule over state can
separate them. The netting cannot, the Adj-RIB-Out cannot, and "does the peer
hold it" cannot. What separates them is when they happen, so the rule that
answers it lives on the connection and not in either table.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-06 | spec-fixit-api-batch-reaches-the-wire-unnetted | the batch-netting rule table in `docs/architecture/exabgp-bridge.md`, written by the same work that added `Net` (`internal/exabgp/bridge/bridge_batch.go`) | The table read "`announce X` then `withdraw X` puts neither on the wire". `Net` cancels the announce alone and keeps the withdrawal, and whether that withdrawal reaches the wire is `withdrawBatchFromPeers`'s answer (`internal/component/bgp/reactor/reactor_api_batch.go`), which writes it for every connection that has advertised something. So the sentence holds for `api-fast` batch 1, the case in front of the author, and is false for every batch after a session's first announce, which is the ordinary case. An operator reading it would predict one frame where ze sends two | fixed in closure: each row now states what the batch DISPATCHES, and a paragraph above the table names the connection-state guard and routes to `docs/architecture/update-building.md`, "A Withdrawal Names a Route This Connection Advertised". Transferable: the netting and the guard are two layers because the fixture needed a TEMPORAL distinction, and A-5 of that spec broke for the same reason: it put the state on the API rail's own Adj-RIB-Out, which would have dropped a legitimate withdrawal from a peer holding config-declared routes |
