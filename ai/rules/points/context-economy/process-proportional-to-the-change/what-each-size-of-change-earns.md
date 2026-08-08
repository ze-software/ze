---
kind: table
level:
stage:
---
| The change | What it earns |
|------------|---------------|
| A few lines inside one function, no new symbol | The main thread. One review pass. No implementation agent: the agent's startup floor costs more than the edit |
| One file, one concern, no new exported symbol | One implementation agent or the main thread. One review pass |
| Several files, a new exported symbol, or a new code path | One implementation agent. One review round, then further rounds only while a round finds a BLOCKER or an ISSUE in its own scope |
| A new subsystem, a protocol change, or anything carrying an RFC or interop obligation | The full loop in `ai/rules/planning.md`, unbounded in passes. That sits on rung 2 of `ai/rules/rule-precedence.md` and no budget reaches it |
