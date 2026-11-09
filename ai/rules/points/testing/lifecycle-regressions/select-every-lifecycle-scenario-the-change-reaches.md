---
kind: directive
level: MUST
stage:
---
- **A change to plugin start-up, plugin supervision, a plugin's failure handling, config reload or daemon stop MUST select every scenario in the table below that the change can reach, and MUST prove each selected scenario with a test that asserts the outcome its page declares.** The outcome is the plugin's resulting state, the warning or refusal the operator sees, and whether the daemon keeps running. A test that checks only that the daemon is still up proves nothing about the scenario.
- **The proof MUST be a `.ci` test through the daemon, and a scenario whose interleaving a `.ci` cannot reproduce reliably MAY use a controlled seam in a Go test instead.** The spec names each scenario that took the seam, and why.
- **Where the plugin runs both internal and external, each selected scenario MUST run in both modes.** The external mode crosses a process boundary that the internal mode never does, so a pass in one mode says nothing about the other.
- **The spec MUST name each scenario in the table that the change cannot reach, with the reason.** An unnamed scenario reads the same as a forgotten one.
