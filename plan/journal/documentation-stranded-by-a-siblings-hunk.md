# Documentation stranded by a sibling session's hunk

A change makes a page wrong, so the work that changed the behavior writes the
page edit in the same piece of work, which is what `ai/rules/documentation.md`
requires. Then the commit cannot carry it: another session already holds an
uncommitted hunk in the same file, and `./le commit create` stages whole files,
so naming the page would carry that session's work into this commit.

The edit is written, correct and verified against its producers, and it lives
nowhere but one working tree. It dies at the next clean, checkout or machine.
Nothing is red, no gate fires, and the spec that owed the page is closed and
gone, so the only thing that remembers the page is owed is a row like this one.

The tell is a documentation checklist row that can be answered neither Yes nor
No: the text exists, and the commit that should carry it cannot.

Two repairs exist and both are outside the closing session's reach. The sibling
session commits its hunk, after which one small commit lands the page. Or the
owner authorizes the closing commit to carry the whole file. Neither is a
decision the closing agent may take on its own, because the ban on carrying
another session's work is in `ai/INSTRUCTIONS.md` and has no author-side
exception.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-06 | ze-test-dns-stub | `docs/functional-tests.md`, the `### ze-test dns` section and the `caps=` token table | The spec's only documentation obligation was one page. The section is written: the zone table matches `defaultZone` (`internal/test/mock/dns/zone.go`), the NODATA paragraph matches `handle` (`internal/test/mock/dns/server.go`), and it carries three `<!-- source: -->` anchors. It could not be committed because a session working on the ExaBGP stage holds an uncommitted hunk in the same file, pinning `exaBGPPythonPackage`, which is absent from HEAD. Both closure commits therefore leave the page unchanged and the text uncommitted | Not fixed. Recorded rather than forced: staging the page would carry the other session's work, and rewriting the file to drop their hunk would destroy it (`ai/rules/never-destroy-work.md`). The page lands when the ExaBGP session commits, or when the owner authorizes one commit to carry the whole file. Until then `git diff -- docs/functional-tests.md` is the only place the text exists |
| 2026-09-06 | firewall-domain-group | `docs/guide/command-reference.md`, the `### show, update and clear firewall domain-group` section | Second row of this class in one day, on a different page and a different sibling. The section is written and correct: three commands checked against the `sdk.CommandDecl` names in `runFirewallDomain` and against `ze-firewall-domain-group-cmd.yang`, placed beside the `firewall irr` section it is modelled on. The same file carries an uncommitted `ze bgp decode pcap` hunk from the session that landed the pcap reader, plus three further hunks, so naming the file would carry their prose. The other five documentation obligations of this spec are committed, so the checklist reads Yes on every row but this one | Not fixed, and the repair is the same as the row above: the pcap session commits, or the owner authorizes one commit to carry the whole file. Worth noting that the recurrence is not chance -- both stranded pages are the two files EVERY spec touches, `docs/functional-tests.md` and `docs/guide/command-reference.md`, so the collision rate rises with the number of sessions rather than with the size of the change |
