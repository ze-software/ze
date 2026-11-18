---
kind: directive
level: MUST
stage:
---
**On a harness that runs Anthropic's models, every spawn MUST name the model by the kind of work the agent is given, whatever model the main session runs on: the strongest tier for design and for the creative parts of implementation, the development tier for legwork.** A spawn that names no model inherits the session's, so the main thread MUST cut the work so that each agent holds one kind; what counts as each kind, and how a spec's tier row overrides it, is `ai/META-DECISIONS.md`, "Delegate to subagents". A harness that runs another vendor's models ignores the split. Review still requires a context independent of the author, as specified below.
