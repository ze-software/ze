---
kind: table
level:
stage:
---
| Question | If YES | If NO |
|----------|--------|-------|
| Would an operator change this during normal capacity planning or traffic engineering? | YANG config | Keep reading |
| Does it need validation, commit/rollback, or config diff? | YANG config | Keep reading |
| Should it appear in `show configuration` or config backups? | YANG config | Keep reading |
| Is it a debug, emergency, or development-only knob? | Env var only | YANG config |
| Is it needed before config loads (bootstrap)? | Env var only | YANG config |
| Is it a safety cap that should never be tuned in production? | Env var only | YANG config |
