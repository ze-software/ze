# Contributor guide

This guide explains how to propose a change to Ze, set up a build, and prepare
the change for review. The [contribute page](../) explains how the project is
funded and what its Contributor License Agreement grants.

Everyone taking part is expected to follow the
[Code of Conduct](../../code-of-conduct/).

## AI-assisted contributions

AI-assisted contributions are welcome. We recommend recent, capable models.
The [contribution policy](https://github.com/ze-software/ze/blob/main/CONTRIBUTING.md#ai-assisted-contributions)
gives the current model examples and the same requirements for specifications,
tests, documentation, and independent review that apply to every contribution.

## How work gets in

An agreed spec records the expected behavior and tests before implementation,
so the author and reviewer can check the change against the same requirements.

1. **Open an issue** describing what you want to change or fix, on the
   [tracker](https://github.com/ze-software/ze/issues). Start here even for
   a fix, so the work is visible and nobody doubles up.
2. **Agree on a spec.** The maintainer turns the issue into a spec that says
   what the change should do and how it will be tested.
3. **Implement the spec**, starting with tests for the behavior it requires.
4. **Arrange independent deep review.** The reviewer uses a context that did
   not produce the change. The [repository guide](https://github.com/ze-software/ze/blob/main/CONTRIBUTING.md#review-and-submission)
   links the review workflow.
5. **Prepare signed-off commits** through the
   [commit-script process](https://github.com/ze-software/ze/blob/main/CONTRIBUTING.md#commits-and-license).
   Each contribution commit requires a `Signed-off-by` line accepting the
   [CLA](https://github.com/ze-software/ze/blob/main/CLA.md).
6. **Open a pull request** linking the agreed spec and giving the resulting
   behavior, scoped test results, documentation changes, and independent review.

## Setting up a build

```bash
git clone https://github.com/ze-software/ze.git
cd ze
./ze --help
```

On a fresh checkout, the launcher builds Ze with the checkout's feature selection
before running the command. Later invocations reuse the existing binary.
The [quickstart](../../guides/quickstart/) covers credentials and gets
two BGP peers talking. For the full developer setup, including test dependencies, see
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

There is no formal good-first-issue program yet. The most useful things an
outside contributor can do right now are the ones that need real routing
experience: run a lab peer, migrate an [ExaBGP config](../../use-cases/exabgp-migration/),
stand up a looking glass, or run the [interop labs](../../labs/) and report
what does not match. Come and ask on [Discord](https://discord.gg/T8s7CjPDne)
where help is most valuable at the moment.
