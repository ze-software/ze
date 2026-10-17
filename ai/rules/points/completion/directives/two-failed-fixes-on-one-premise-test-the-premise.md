---
kind: directive
level: MUST
stage:
---
**When two fixes that rest on the same belief have both failed the same test or gate, you MUST write that belief down in one sentence and test it directly before you write a third fix that assumes it.** Each failed fix is evidence against the belief it assumed, so a third fix on it spends effort where the evidence already says the cause is not. The test is an observation that comes out differently if the belief is false: look at how the failures are distributed, not only how large they are. If the belief is false, the cause is elsewhere, so look for what the failures share that the belief did not predict. If it holds, the earlier fixes treated a consequence, and the next question is what produced it.
