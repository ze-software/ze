# User-Facing Errors Rationale

Why: `ai/rules/user-facing-errors.md`

## The owner's words

The owner's instruction of 2026-10-10:

> make sure we have a clear rule to be user friendly with errors message, this is why we have ze doctor, it is an important part of software design and good errors go a long way for software adoption

The owner's principle (D-7), restated generically by him:

> The host should be set up correctly for Ze. A user will meet a misconfigured host sooner or later; when they do, Ze should report it without friction, and the path from the symptom to understanding the problem and its fix should be as smooth as possible. Where tooling can make sure the admin does the right thing, we should have the tooling.

And his clarification of its scope:

> the point is user friendliness, on every user interface Ze has (CLI, web, `ze doctor`, logs, the `./le` tooling), not the CLI alone, so every refusal names what is wrong, why, and the exact command that fixes it.

And on how a message reads:

> this point include good clear message, well presented and easy to read, using color tastefully to highlight the important points, etc. no jargon, plain english and why we use our simplified technical english rule

And on where the rule lives:

> this MUST be clearly in our repo rules for all agents

## Why a rule of its own

The instruction used to be one sentence inside the CLI rule, so a change to a
web notice, a log line or an `./le` diagnostic never routed to it. A rule of its
own carries a trigger that names every surface.

## The case that prompted it

On a Linux host, from a non-interactive shell, `./le setup docker-kernel check`
answered `cannot run go: exec: "go": executable file not found in $PATH` and
`exited 127`. Every word was true and none told the reader what to do.
`gotoolchain.New` (`internal/le/go/toolchain`), the one place le resolves the Go
toolchain, now refuses before any Go command starts, and the `le` launcher
refuses before it compiles. Both name the PATH searched, the version go.mod pins,
and how to put Go on PATH.

## No mechanical check

No check is both cheap and real. A pattern over error strings ("exit N" alone)
catches only the most literal case and reads as coverage for the rest, so the
rule is a reviewer's check.
