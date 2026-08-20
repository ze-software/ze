# Error path discards data already received

A reader that stores a terminal error and a consumer that checks that error
before the queue are two correct halves of a broken whole. The last bytes a peer
sent arrive, get queued, and are then dropped in favour of the EOF that followed
them. The loss looks like a peer that never sent the data, so it is diagnosed on
the wrong side of the connection. Ask, of every error check placed above a
receive: what is already in the queue when this returns?

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-19 | streaming-answer-protocol | plugin rpc framing | `readFrame` (`pkg/plugin/rpc/conn.go`) opened with a fast path returning `readerErr` when the persistent reader had stopped. Its `readerDone` branch did the same. The reader stores that error one read AFTER it pushed a frame into the capacity-1 `frameCh`. So a peer that wrote its last line and closed the connection lost that line. Every caller was affected. A `CallRPC` whose peer answered and exited at once got `EOF` instead of its answer. A streamed answer lost its last records and its terminator, which is the failure the spec exists to prevent | fixed: the fast path is gone. The `readerDone` branch calls `queuedFrame`, which takes a queued frame before it reports why the reader stopped. `TestAnswerWithoutTerminatorReportsTruncation` (`pkg/plugin/rpc/mux_test.go`) drives a peer that writes a head and one record and then closes. It asserts the record is delivered and the answer reads as truncated. It fails on the old code with zero records delivered |
