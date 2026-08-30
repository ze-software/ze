---
kind: directive
level: MUST NOT
stage:
---
**The `--flag` form belongs to the offline `cmd/ze/` Go flag tooling that reaches no daemon.** It MUST NOT reach the YANG layer, and a client MUST NOT send one to the daemon. A filter is grammar, so it MUST be a keyword-value pair, and every offline flag MUST be declared through the flag registry. Rendering is the pipe layer's job, so a RENDERING flag MUST NOT exist on any command. A command with an answer MUST register that answer through `registry.MustRegisterLocalData`. `--version`, `-V`, `--help` and `-h` are the one exception, additive to `ze version` and `ze help`. `docs/architecture/cli/command-namespacing.md` states which register a token belongs to.
