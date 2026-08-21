---
kind: directive
level: MUST
stage:
---
**The name is symmetric, and it MUST be read that way in both directions.** One declaration says that this plugin READS a record answer and WRITES one. No engine-to-plugin message carries a protocol list, and none MUST be added: the engine reads that one line to write the answer to `dispatch-command` and `dispatch-command-args`, and to read the answer to `execute-command`.
