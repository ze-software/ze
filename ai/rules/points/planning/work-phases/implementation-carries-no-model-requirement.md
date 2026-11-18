---
kind: directive
level: MUST
stage:
---
**Every phase agent MUST be spawned on the development tier, with the model named in the spawn, whatever model the main session runs on; only a phase whose spec row names the strongest tier runs on it.** A spawn that names no model inherits the session's, so a session on the strongest tier silently puts every agent there (`ai/META-DECISIONS.md`, "Delegate to subagents"). Review still requires a context independent of the author, as specified below.
