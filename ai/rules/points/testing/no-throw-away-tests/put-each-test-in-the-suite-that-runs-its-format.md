---
kind: directive
level: MUST
stage:
---
**A test MUST go in the suite that runs its format.** Each `test/<subdir>/` has
its own runner and format, and they are not interchangeable: `test/parse/`
accepts only config-parse `.ci` files, so a BGP-plugin scenario placed there is
rejected and belongs in `test/plugin/`.
**Pure-logic, reactor-free code (an encoder, a parser, a state machine exercised
directly) MUST be tested in a Go unit test, never in a `.ci` directory.** A `.ci`
exists to prove a user entry point works end to end through the daemon.
