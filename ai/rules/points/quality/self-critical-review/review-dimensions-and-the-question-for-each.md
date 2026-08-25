---
kind: table
level:
stage:
---
| Check | Question |
|-------|----------|
| Correctness | Actually works? Edge cases? |
| Simplicity | Is this the simplest FULLY CORRECT answer? Name every abstraction, option, layer, and parameter the problem in hand did not need (`ai/rules/simplicity.md`) |
| Modularity | Modified files still one-concern? Line count ok? (rules/file-modularity.md) |
| Consistency | Follows existing patterns? |
| Style | Every loop, queue, retry and cache bounded? Every name says what the value IS? Every lifecycle obligation in a comment? No `panic()` a peer can reach? (`docs/contributing/ze-go-style.md`) |
| Completeness | TODOs, FIXMEs, unfinished? |
| Quality | Debug statements removed? Errors clear? |
| Tests | Cover the change? Any flaky? |
