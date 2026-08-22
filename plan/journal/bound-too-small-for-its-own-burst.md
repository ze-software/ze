# A bound sized independently of the burst its own design produces

One half of a design decides how many items are handed over at once. Another
half decides how many items can be held for a consumer that has not been
scheduled yet. When the two numbers are chosen apart, the second can be smaller
than the first. The handover then fails for a consumer that is merely late
rather than for one that is stuck. The failure is load-dependent, so it is
invisible in a quiet checkout and arrives as a flake in somebody else's suite.

The tell is two constants in different files that name the same quantity, and a
comment on the bound asserting that it is generous. Derive one from the other,
or state in both places why they are allowed to disagree. A test written over
such a path must stop inside the bound rather than pace the producer, because
pacing is a property of the transport and the scheduler and cannot be asserted.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-22 | - | `answerQueueDepth` (`pkg/plugin/rpc/mux.go`), the queue an engine reads one plugin answer through | a streamed answer opens with a head, then the whole buffering window in one burst, then the rest. That window is `AnswerBufferThreshold` records, so the shortest streamed answer is 259 lines: a head, 257 records and a terminator. The queue holds 256, and `readLoop` abandons the answer past that rather than wait, so EVERY streamed plugin answer can be cut short by a consumer that has not been scheduled yet. The queue's own comment says it is deep enough that such a consumer never trips it. Measured on 2026-08-22 under `scripts/dev/stress-repro.py` with 32 burners on 16 cores, over a 300-row plugin answer: 1 abandonment in about 30 invocations at 48 bytes a row, and 1 in about 28 at 4096 bytes a row, each with `mux conn: answer queue full, abandoning answer id=5 depth=256` in the daemon log and `plugin answer ended before its terminator` reaching the operator as a rejected row. Widening the rows only moves the odds, because it paces the producer through the socket buffer rather than removing the race. `test/plugin/plugin-owned-command-streams.ci` survives because its rows are 60000 bytes, so 18 MB of socket writes pace the walk; it is not evidence the bound is sound | NOT FIXED. Recorded rather than repaired: the repair is a flow-control decision, and `readLoop` serving every id on one connection is why AC-17 and R-5 of spec-record-answers-1-sdk-path forbid it to wait on one consumer. Raising the depth moves the threshold and does not remove the race. `test/plugin/stream-answer-renders-table.ci` reads its answer through `first 100` and `first 5`, which stop inside the 256 the queue guarantees, so the test measures the column schema it was written for and never the queue |
