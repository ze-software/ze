# The repository is half the AI harness

*2026-08-09 by Thomas Mangin*

An agent needs a way to find the project's decisions before it edits, and useful feedback when it gets them wrong. Maintaining both takes human time.

![General harness abilities and repository-specific meaning converge on an agent change; checks return failures to the rule and example that guide the next revision.](../../assets/blog/the-repository-is-the-ai-harness.svg)

Ze is a network operating system spread over 770+ Go packages. Being able to open a file gives an AI agent very little help in deciding which package a change belongs in. It needs to find the design decision behind the code before it adds another plausible implementation of the same idea somewhere else.

The harness provides general tools for reading, editing and running programs. I have been putting the project-specific guidance in the repository, because a correction in one conversation is gone when the next session starts. A task index can direct an agent to the rule and an existing example, while a check can catch some of the mistakes it makes despite reading them.

I argued in [AI slop is the wrong test](../ai-slop-is-the-wrong-test/) that I remain responsible for the generated code. The practical difficulty is giving an agent enough information to avoid repeating a mistake without burying it in everything the project knows.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture, design decisions and conclusions come from my work on Ze. The models helped organise the material and draft the text.*

## Finding where a plugin belongs

A plugin change starts at [`ai/INDEX.md`](https://github.com/ze-software/ze/blob/main/ai/INDEX.md). Its feature table points to the [plugin pattern](https://github.com/ze-software/ze/blob/main/ai/patterns/plugin.md), then to the plugin rules and goroutine-lifecycle rules. The pattern gives the expected file layout and links to the core architecture. The agent has a route into the relevant material before it needs to search for an implementation.

Registration is one decision it finds there. A plugin declares itself to a registry from a `register.go` file. The core can then discover the plugin through the registry instead of carrying an import and a dispatch branch for each extension. A central switch is reasonable for a closed set of commands, but making every new plugin edit that switch would defeat the separation I want.

The [registration pattern](https://github.com/ze-software/ze/blob/main/ai/patterns/registration.md) explains the mechanism, and the [Graceful Restart plugin's registration](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/register.go) is a concrete example. It declares the plugin name `bgp-gr`, its entry point and the capabilities it provides. Its `init()` function calls `registry.Register`, which rejects invalid registrations such as a duplicate name or a missing engine entry point.

`init()` is part of the approved pattern. The rule is about keeping registration in the file where the project expects to find it. The [import generator](https://github.com/ze-software/ze/blob/main/internal/le/plugin/imports/pluginimports.go) scans the plugin roots for `register.go` and derives the packages the binary must load. `./le repository generate` includes that generation step. The agent therefore has both an example to copy and a reason why the filename matters.

That sequence is more useful than an instruction to respect the architecture. The agent can follow a task to a decision, inspect how an existing plugin satisfies it, and see which part of the build depends on the convention.

## What the rejection tells the agent

The [Go edit hook](https://github.com/ze-software/ze/blob/main/internal/le/hookruntime/writeedit.go) looks for registration-like calls near the start of an `init()` function and exempts files whose names begin with `register`. When it recognises that pattern in an ordinary implementation file, it refuses the edit. The diagnostic says `BLOCKED: Implicit behavior in init()`.

That wording is less helpful than it should be. Taken literally, it contradicts the example the agent just read, because the example uses `init()` too. The current diagnostic does not include the explanation and link which the earlier hook printed. The useful correction is to put registration in `register.go`, with a reference back to the registration pattern, so the agent can repair the location without removing the mechanism.

Whatever a check prints becomes part of the agent's next prompt. If it only says that a pattern is forbidden, the model can satisfy it by choosing a different spelling while preserving the mistake. I want the failure to name the affected file and explain the accepted implementation, with a link for the reasoning which would make the message too long.

Catching the mistake during the edit also limits how much code can be written on top of it. A later compiler or test failure is still useful, but by then the agent may have repeated the same decision across several files. Early checks pay for themselves when they recognise a narrow mistake and explain it accurately.

The indexes serve the other direction. `ai/PACKAGE-MAP.md` gives package descriptions, while `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` provide the links between implementation and documentation. An agent which arrives through a source file can look up its explanation; one which arrives through a design can find the implementing files. I want that route to remain available after the authoring session has ended.

## Finding the right test is another navigation problem

The task index also points to [`ai/rules/testing.md`](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) and the functional-test documentation. A plugin API change needs an observation through the running program. An internal parser test can help isolate a defect, but it cannot show that the plugin was loaded or that the daemon delivered the request to it.

Writing down the required test surface avoids asking the agent to choose the cheapest test it can turn green. It does not remove the harder question of whether the selected test reaches the behaviour it names.

In August, three redistribution scenarios stayed green with their late-join replay disabled. The route reached the peer through another path, so observing the route did not establish that replay had happened. The tests looked relevant and ran through the program, yet the defect they were supposed to catch could not make them fail.

That led to deliberately breaking the behaviour a new test claims to protect and checking whether the test notices. A Claude session started doing this without being asked. It costs another rebuild and run, and it can expose a fixture which needs to be redesigned before any implementation can be trusted. I have never regretted that expense.

There was a different failure in the BGP reactor tests. Three tests maintained their own map of which address families had been sent an End-of-RIB, although production had no such map. A session reading them nearly reported a conformance violation in code that did not exist. A proposed detector was too noisy because ordinary table-driven tests also build local data, so the replacement was a reading habit: identify the production function and check that the test calls it.

These incidents changed what the agent is asked to look for. They did not produce a general check which can recognise a meaningful test. [The proof is the expensive part](../the-proof-is-the-expensive-part/) follows one RFC requirement through its tests and public evidence, including the limits of what those tests observe.

## When the check creates its own problem

The test-edit hook was meant to stop an agent fixing a red test by weakening its assertions. Humans do that too, usually more slowly and with better excuses. A legitimate relaxation needs a reason, but the reason is useful only if someone can distinguish it from the paperwork required to get past a bad check.

An early version counted non-comment lines in a scenario. Replacing several sleeps with one wait condition reduced the line count and was refused, while replacing assertions in place could leave the count unchanged. The check penalised a timing repair and could miss a loss of coverage.

The [10 August record](https://github.com/ze-software/ze/blob/main/plan/journal/hook-existing-patterns-false-positive.md) counted 751 `test-relax:` tokens across 466 files, including 362 in scenario files excusing timing refactors. Most of those comments were receipts for getting past the hook. Separating the genuine concessions from them took an audit I should not have needed.

The counter was changed, but the same journal records another false positive two days later: combining peer processes kept the assertions and reduced the command count, so the revised check objected again. A rule can be wrong in several successive implementations. Each refusal costs a detour, and repeated false alarms teach an agent to supply the escape text as routine.

Those inline markers describe the older workflow. The current rules record approved test changes in per-session ledgers, and the [native edit hook](https://github.com/ze-software/ze/blob/main/internal/le/hookruntime/writeedit.go) reads the proposed edit through the test-weakening checker. Changing an expected value in place or repointing a test at different behaviour can still preserve every structural count. I still have to judge what has been given up.

## Keeping the repository useful

I would rather not pretend this machinery is free. Every moved design and renamed source file can leave guidance pointing at the wrong place. Generated indexes save us from maintaining large tables by hand, but someone still has to correct the underlying references and decide which document explains a change.

The checks themselves need the same attention as the code they judge. A bad rule can be copied into many files by an agent doing exactly what it was told, and the resulting consistency makes the mistake harder to recognise. Maintaining a repository for agents includes removing checks which cost more in false refusals than they catch in defects.

A smaller project could start with one task-oriented index and the patterns its contributors repeatedly get wrong. A reliable check can follow once the mistake is narrow enough to recognise. The useful unit is the route from a common task to its explanation and example, then back from a failure to the correction. Copying Ze's entire verification system would also copy the maintenance burden which produced it.

I discuss the case for shared conventions in [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/). This account began in summer 2026 and has already needed corrections as Ze's tools changed. A more capable model may need less instruction, or may use a richer repository correctly. I would want to know which of these failures still occurs before adding another rule to deal with it.

*Last updated: 8 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-repository-is-the-ai-harness.md).*
