---
kind: note
level:
stage:
---
The hook's refusal of the bare command is deliberate, not an oversight: it is
what forces every push through the script, where the mechanism stays visible to
the next reader. Writing a throwaway script that carries a push and deleting it
afterwards is banned for the same reason. It reaches the same remote by the same
hand and leaves no record of why the push happened, and `ai/INSTRUCTIONS.md`
carries that ban into every session. A remote history that needs rewriting is
the owner's decision, made at his own terminal. `./le commit create` has no
force keyword, and the `push "<owner authorisation>"` route never rewrites
remote history (`ai/INSTRUCTIONS.md`, "Destructive git commands are FORBIDDEN").
