# Contributing to Ze

Bug reports and contributions are welcome. By contributing, you agree to the
[Contributor License Agreement](CLA.md).

## AI-assisted contributions

AI-assisted contributions are welcome. You may use AI to research a problem, develop and test a solution, and prepare a pull request.

We prefer recent, capable models such as Sol 6.1, Astra, Opus 5.5 or Fable 5.2, and newer models of comparable capability. This is a recommendation, not a requirement.

Every contribution must meet the same specification, testing, documentation and independent review requirements.

## Reports and questions

The [issue tracker](https://github.com/ze-software/ze/issues) accepts bug reports,
ExaBGP migration reports, and documentation corrections. A useful report names
the Ze commit or build, describes the expected and observed behavior, and gives
the smallest redacted example that reproduces the problem.

Questions about using Ze can go to [Discord](https://discord.gg/T8s7CjPDne).
Security reports use the private contact in [SECURITY.md](SECURITY.md).

## Before implementation

Open an issue describing the problem or proposed change. The maintainer creates
an agreed spec from [the spec template](plan/templates/TEMPLATE.md) before
implementation starts. The spec records the behavior, tests, and documentation
that the contribution must deliver.

The rules in [ai/rules/](ai/rules/) apply to contributions written by hand and
with AI assistance. [The rule index](ai/rules/TRIGGERS.md) names when each rule
applies; [ai/INDEX.md](ai/INDEX.md) points to the relevant documentation.

## Implementation and evidence

Write tests before implementation, and include the code, tests, and documentation
needed to complete the agreed spec. Tests must exercise the behavior the change
claims to provide. Existing tests must retain their coverage; changes to
RFC-tagged tests and changes that could weaken a test require independent
judgment through [the test review process](docs/contributing/testing.md).

Run the tests of the packages and functional suites the change reaches, directly
in the main checkout. For one package, the scoped command is:

```sh
./le test unit pkg path ./internal/core/textbuf/
```

Replace the example path with the package being changed. The command uses the
checkout's feature tags and race detector. The [testing guide](docs/contributing/testing.md)
explains functional tests and changes that need wider coverage. For agent-assisted
contributions, the main session runs the commands reported by implementation agents.

Report the exact commands and results, including failures, skipped tests, and
checks that could not run. GitHub CI/CD verifies pushed commits. Contributors
do not start the full `./le verify current` release check.

## Review and submission

Arrange a deep review before submission using
[the ze-review-deep workflow](ai/skills/ze-review-deep.md). The reviewer must use
a context that did not produce the change.

The pull request should link the agreed spec, explain the resulting behavior,
and include the test evidence and review findings. A contribution is complete
when its agreed code, tests, and documentation are complete.

## Commits and license

Use the [commit-script process](docs/contributing/committing.md):
`./le commit create` prepares a message and script, and the printed `script=`
path identifies the script to run. Read the generated message before running it.

Each commit must include `Signed-off-by: Your Name <your email address>` with
the contributor's own name and address. Add that line to the generated message
before running the script. The sign-off signifies acceptance of the [CLA](CLA.md),
as its "How to Accept" section specifies. The repository's commit-signing
requirements also apply.

Ze is licensed under the [GNU Affero General Public License v3](LICENSE).
The CLA grants the maintainer the right to offer the project under
additional license terms (dual licensing). Your contributions remain
available under AGPL-3.0 to the public at all times.
