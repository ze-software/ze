# Contributing to Ze

Contributions are welcome if they follow the process below.

By contributing, you agree to the terms of the
[Contributor License Agreement](CLA.md).

## Process

1. **Open an issue** describing what you want to work on.
2. **Wait for a spec.** The maintainer will create a spec from `plan/templates/TEMPLATE.md`
   so the work follows the project's structure and standards.
3. **Implement the spec.** Follow the rules in `ai/rules/` -- they apply to
   all contributions, whether written by hand or with AI assistance.
4. **Run `/ze-review-deep` before submitting.** The maintainer reviews contributions
   with Claude Code. Submitting pre-reviewed code makes that process smooth.
   See [`ai/INDEX.md`](ai/INDEX.md) for the repository's command and rule index.
5. **Sign your commits** with `git commit -s` (certifies you accept the [CLA](CLA.md)).

## Rules

The `ai/rules/` directory contains the project's coding standards, testing
requirements, and workflow expectations (`ai/rules/TRIGGERS.md` names when
each one applies). Key points:

- **TDD:** tests written before implementation
- **Specs drive work:** no code without a spec
- **Test what the change reaches:** run the tests of the packages and
  functional suites your change touches. The full verification
  (`./le verify current`) is the release check that GitHub CI runs on a pushed
  commit; a contributor does not run it
- **No partial deliveries:** code + tests + docs in one piece

## License

Ze is licensed under the [GNU Affero General Public License v3](LICENSE).

The CLA grants the maintainer the right to offer the project under
additional license terms (dual licensing). Your contributions remain
available under AGPL-3.0 to the public at all times.
