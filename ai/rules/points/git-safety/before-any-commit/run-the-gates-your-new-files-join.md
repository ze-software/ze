---
kind: directive
level: MUST
stage:
---
**A gate's population is defined by where a file LIVES, not by what you edited, so
before commit you MUST run the repo-wide counters, inventories and ratchets whose
population your NEW files join.** Adding a file to a directory such a gate walks
puts you inside it, even when the gate was written for a concern your change has
nothing to do with, and even when every gate for the surface you edited is green.

**Scoped evidence is keyed on the surface; the gate that catches you is keyed on
the directory.** That asymmetry is why a careful, fully verified commit can still
turn a shared gate red for every other session: the author ran what their change
was about, and the gate counts what their change added.

**Ask it as a question about paths.** For each path in the commit that did not
exist before, name the repo-wide checks that walk its directory, and run them. A
new test fixture, a new scenario, a new script and a new generated artifact are
the usual carriers, because each lands in a tree something else counts.
