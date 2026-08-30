---
kind: directive
level: MUST
stage:
---
A token MUST be placed in its register by these tests, applied in order, first yes wins:

| Test | Register | Form |
|------|----------|------|
| Does it change WHICH answer is produced: the object, the sub-section, the selector, the filter, the variant? | Command grammar | bare keyword, declared in YANG or a `CommandDecl` |
| Does it change how an answer already in hand is rendered or reduced? | Pipe operator | `\| json`, `\| count`, `\| match` |
| Does it change how this process starts, before any command exists: which config, which daemon, which credentials, which plugins, which listener, which colors? | Process option | `--flag` |
