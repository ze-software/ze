---
kind: directive
level: MUST
stage:
---
**A closure MUST end in one of two states: closed, with commits A and B landed, or handed back. A spec MUST NOT stay `in-progress` once its owner has stopped.** A closing agent that cannot finish MUST, before it stops, set `| Status | blocked |`, write the blocker into the spec (what blocks, the producing file and symbol, what clears it), commit through `./le commit create`, and run `./le spec release`. The main thread sets the spec back to `in-progress` when the blocker clears. `/ze-close` carries the procedure.
