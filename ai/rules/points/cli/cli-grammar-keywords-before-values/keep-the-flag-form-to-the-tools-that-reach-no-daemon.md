---
kind: directive
level: MUST
stage:
---
- **The `--flag` form MUST stay in the offline `cmd/ze/` tools that reach no daemon and have no pipe layer: `appliance`, `analyze`, `perf`, `chaos`, `le`, `install`, `provision` and the mock servers.** These are build, lab and analysis tools, not the router's operator language.
- **A command that crosses to the daemon MUST take keywords, on every front end an operator can type it from.** The SSH CLI, the web terminal, `ze cli -c` and the offline `ze <verb>` dispatch all reach the same command, so one spelling MUST serve all four.
- **MUST NOT give any command a flag spelling, with one exception: `--version`, `-V`, `--help` and `-h`, which every Unix program answers.** A person meeting `ze` for the first time types one of the four before any help exists to tell them otherwise, so `ze` answers them.
- **The four are ADDITIVE, never a replacement: `ze version` and `ze help` MUST exist as commands and MUST stay the canonical form.** The command is what the tree declares, what completion offers and what the documentation names; the flag answers identically beside it. This is a Unix convention, not a compatibility shim, so `ai/rules/go-standards.md` "No Backwards Compatibility" does not reach it and nothing else joins the set.
