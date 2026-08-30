---
kind: directive
level: MUST NOT
stage:
---
**Each reason below MUST NOT be used to keep a phase in the main thread.**

| Banned | Reality |
|--------|---------|
| "This edit is small, I will just do it inline" | Size is judged after review. A one-line spec change still passes through the phase that owns it |
| "Spawning an agent costs a round trip" | The round trip is the supervision. Doing the work inline is what the main thread is not for |
| "I already have the context loaded, an agent would have to re-read it" | Re-reading is cheap; a main thread that fills with implementation detail cannot supervise the phases that follow |
| "The agent's report looks right, I will pass it on" | Unverified relay is fabrication with an extra hop (`ai/rules/evidence.md`) |
| "I will implement it and then spawn a reviewer" | The implementation phase was owed a subagent too. One rule broken does not excuse the next |
| "This grep is quicker if I run it here" | Exploration in the main thread is the spend supervision exists to avoid |
| "My package is too big, so I will cut the last acceptance criterion" | The boundary was chosen at decomposition. Report the size and let the main thread re-cut it; scope reduction is the user's call (`ai/rules/completion.md`) |
