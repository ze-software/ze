# Guard addition drops what it refuses

A guard added to enforce one obligation turns an unconditional send into a
conditional one. The refused branch has to do something, and `return` is the
cheapest thing to write. It is also a silent drop, and a drop is usually a
different obligation broken on the same path. Ask what the refused input owed
before you decide what the refusal does with it.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-12 | fixit-ike-request-window | ike engine | the one-request window added for RFC 7296 Section 2.3 made `sendDeleteIKE` and `sendDeleteESP` (`engine/delete.go`) return silently on a held window, where they had sent unconditionally. That broke Section 2.1's enrolled MUST to remember a request until its response arrives, and left the peer holding an SA Ze had torn down. Three reviewer lenses also refused the first two repairs. One was a deferral queue no production caller can drain. The other freed the window before the goodbye, which is the same Section 2.3 violation with local bookkeeping in front of it | `PeerSession.writeDelete` arms `SA.armRequestRetransmit`, so the window's own retransmission carries the Delete. `PeerSession.sayGoodbye` (`engine/established.go`) waits `goodbyeWindowWait` for the outstanding answer instead of abandoning it, and `PeerSession.serviceRequestWindow` deems the SA failed rather than forgetting the request. The queue was deleted |
