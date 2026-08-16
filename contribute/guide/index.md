# Contributor guide

This is the practical side of contributing to Ze: how work gets in, how to
set up a build, and what the project expects of a change. The
[contribute page](../) covers the why, the funding, and the Contributor
License Agreement. Read that first, then come back here.

Everyone taking part is expected to follow the
[Code of Conduct](../../code-of-conduct/).

## How work gets in

Ze is built spec by spec, and that shapes how contributions flow:

1. **Open an issue** describing what you want to change or fix, on the
   [tracker](https://github.com/ze-software/ze/issues). Start here even for
   a fix, so the work is visible and nobody doubles up.
2. **Agree on a spec.** For anything beyond a small fix, the maintainer turns
   the issue into a spec that says what the change should do and how it will
   be tested. Specs keep the project coherent and stop work drifting.
3. **Implement the spec**, with tests written alongside the code, not after.
4. **Sign your commits** with `git commit -s`. That sign-off certifies you
   accept the [CLA](../). No signed-off commit, no merge: this is the one hard
   requirement.
5. **Open a pull request.** The maintainer reviews it, often with AI
   assistance, the same way the rest of the project is built.

## Setting up a build

```bash
git clone https://github.com/ze-software/ze.git
cd ze
make build          # builds bin/ze
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
- **`make ze-verify` passes.** That is the gate the maintainer runs, so run it
  yourself before you submit.
- **It arrives in one piece.** Code, tests, and documentation together, not a
  code change now with the docs to follow.
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
