# Contributor guide

This guide explains how to propose a change to Ze, set up a build, and prepare
the change for review. The [contribute page](../) explains how the project is
funded and what its Contributor License Agreement grants.

Everyone taking part is expected to follow the
[Code of Conduct](../../code-of-conduct/).

## How work gets in

For a contribution beyond a small fix, we agree on a spec before implementation
so that the expected behavior and tests are clear to both author and reviewer.

1. **Open an issue** describing what you want to change or fix, on the
   [tracker](https://github.com/ze-software/ze/issues). Start here even for
   a fix, so the work is visible and nobody doubles up.
2. **Agree on a spec.** For anything beyond a small fix, the maintainer turns
   the issue into a spec that says what the change should do and how it will
   be tested. Specs keep the project coherent and stop work drifting.
3. **Implement the spec**, starting with tests for the behavior it requires.
4. **Sign your commits** with `git commit -s`. That sign-off certifies you
   accept the [CLA](../). Every merged contribution requires that sign-off.
5. **Open a pull request.** The maintainer reviews it, often with AI
   assistance, the same way the rest of the project is built (the
   [FAQ](../../faq/#ai-development) explains how).

## Setting up a build

```bash
git clone https://github.com/ze-software/ze.git
cd ze
go build -o bin/ze ./cmd/ze
bin/ze init         # set up local credentials
bin/ze cli          # connect to the CLI
```

The [quickstart](../../guides/quickstart/) takes this further and gets two
BGP peers talking. For the full developer setup, including the test
dependencies, see
[developer-setup.md](https://github.com/ze-software/ze/blob/main/docs/guide/developer-setup.md)
in the repository.

## What a good change looks like

- **Tests come with it.** Ze is test-driven; a change without tests is not
  finished.
- **The relevant tests have run.** Run the tests for the packages and behavior
  the change reaches, and include their results with the submission. GitHub CI
  runs the full verification on pushed commits.
- **It arrives in one piece.** Include the code, tests, and documentation in
  the same change so the instructions describe the behavior under review.
- **It respects the project's standards.** The coding, testing, and workflow
  rules the project follows live in
  [the repository](https://github.com/ze-software/ze/blob/main/CONTRIBUTING.md)
  and apply to every contribution, whether written by hand or with AI help.

## Good first paths

There is no formal "good first issue" label yet. The most useful things an
outside contributor can do right now are the ones that need real routing
experience: run a lab peer, migrate an [ExaBGP config](../../use-cases/exabgp-migration/),
stand up a looking glass, or run the [interop labs](../../labs/) and report
what does not match. Come and ask on [Discord](https://discord.gg/T8s7CjPDne)
where help is most valuable at the moment.
