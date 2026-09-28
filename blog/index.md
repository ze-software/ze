# The Ze blog

Design notes, deep dives, and engineering essays on how Ze is built. For week-by-week shipping notes, read the [changelog](../project/changes/).

- [Keeping documentation in step with the code](reference-from-the-system/index.md) (2026-08-22): Ze generates command, configuration and support reference data from the code and its records, so the website does not depend on a second list being updated by hand.
- [AI coding has not had its Rails moment](ai-coding-has-not-had-its-rails-moment/index.md) (2026-08-10): AI coding needs conventions that connect documentation and design decisions to the code they govern. Each project still has to build that structure for itself.
- [The repository is half the AI harness](the-repository-is-the-ai-harness/index.md) (2026-08-09): Ze's rules give AI agents explicit objectives and ways to check their work against the project's requirements, so they can recognise incomplete or incorrect implementations.
- [The proof is the expensive part](the-proof-is-the-expensive-part/index.md) (2026-08-06): Passing unit tests and CI does not establish that Ze behaves as the protocol requires. Our RFC testing framework connects each requirement to the behaviour its tests observe.
- [How Ze reuses memory for BGP UPDATEs](how-ze-manages-memory/index.md) (2026-08-04): Go's garbage collector reclaims unused memory, but creating and copying data still has a cost. Ze's BGP UPDATE processing shows how we design to avoid that work.
- [AI slop is the wrong test](ai-slop-is-the-wrong-test/index.md) (2026-08-03): AI often writes better code than I would, and considerably more tests per feature. I have little patience for calling it all slop when human-written software has given us so much to complain about.
