# Contributing to Ze

Contributions are welcome if they follow the process below.

By contributing, you agree to the terms of the
[Contributor License Agreement](CLA.md).

## Process

1. **Open an issue** describing what you want to work on.
2. **Wait for a spec.** The maintainer will create a spec from `plan/TEMPLATE.md`
   so the work follows the project's structure and standards.
3. **Implement the spec.** Follow the rules in `.claude/rules/` -- they apply to
   all contributions, whether written by hand or with AI assistance.
4. **Run `/ze-review-deep` before submitting.** The maintainer reviews contributions
   with Claude Code. Submitting pre-reviewed code makes that process smooth.
   See the [Claude Code cheat sheet](docs/claude-code-cheatsheet.md) for all available commands.
5. **Sign your commits** with `git commit -s` (certifies you accept the [CLA](CLA.md)).

## Rules

The `.claude/rules/` directory contains the project's coding standards, testing
requirements, and workflow expectations. Key points:

- **TDD:** tests written before implementation
- **Specs drive work:** no code without a spec
- **`make ze-verify` must pass** before any submission
- **No partial deliveries:** code + tests + docs in one piece

## License

Ze is licensed under the [GNU Affero General Public License v3](LICENSE).

The CLA grants the maintainer the right to offer the project under
additional license terms (dual licensing). Your contributions remain
available under AGPL-3.0 to the public at all times.
