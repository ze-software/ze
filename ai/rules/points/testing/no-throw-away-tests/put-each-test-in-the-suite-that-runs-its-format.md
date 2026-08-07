---
kind: note
level:
stage:
---
Each `test/<subdir>/` has its own runner and format, and they are not interchangeable. `test/parse/` only accepts config-parse `.ci` files (config text + `expect=exit:code=`). Putting a BGP-plugin scenario there will be rejected; put it in `test/plugin/`. Pure-logic, reactor-free code (encoders, parsers, state machines exercised directly) belongs in Go unit tests (`internal/<pkg>/<file>_test.go`), not in any `.ci` directory: `.ci` tests exist to prove a user entry point works end-to-end through the daemon.
