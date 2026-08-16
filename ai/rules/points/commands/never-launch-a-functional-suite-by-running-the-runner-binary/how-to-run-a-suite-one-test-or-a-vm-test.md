---
kind: table
level:
stage:
---
| Want | Use |
|------|-----|
| A whole suite | `make ze-functional-plugin-test` (or `ze-functional-encode-test`, `ze-functional-parse-test`, ...) |
| One test, iterating | the make target's own invocation: build the isolated pair with its tags, symlink them bare-named, export `ZE_BIN`/`ZE_TEST_BIN` |
| One test in the VM | `make ze-qemu-debug RUN='...'` -- flags BEFORE positional ids (`-v 145`, not `145 -v`) |
