---
kind: directive
level: MUST NOT
stage:
---
**Simplicity governs the SHAPE of the answer. It MUST NOT govern the EXTENT of its correctness.** Two readings, and the second governs: the first reads "simplest" as permission to do less, with fewer acceptance criteria, fewer RFC MUSTs and a narrower test; the second reads it as the instruction to solve the whole problem with the least machinery.
**The only budget this rule cuts is machinery. It MUST NOT cut correctness, conformance, tests, guards, or error handling, so it is never the reason for a `may I skip it` question, a deferral row, or a partial implementation.**
**The simplest fully correct design is usually the HARDEST one to find, so you MUST budget thinking time for it, and not seeing it MUST NOT be read as a license to ship the complicated shape and call it pragmatic.** When you ship anything other than the most obvious implementation, write one line naming the simpler design and the requirement it failed: an unexplained abstraction reads as habit, and the next reader keeps it because they cannot prove it is unnecessary.
