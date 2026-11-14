---
title: The repository is half the AI harness
date: 2026-08-09
author: Thomas Mangin
description: Ze's development rules define what an AI agent must achieve and how to check the result, including the qualities a working demonstration can leave untested.

deck: The repository's instructions influence how agents implement a feature and decide whether it is finished.

image: assets/blog/the-repository-is-the-ai-harness.svg
image-dark: assets/blog/the-repository-is-the-ai-harness-dark.svg
image-alt: Diagram showing an agent using coding tools and project instructions, then using check results to correct its change.

---

In Ze, the network operating system I am building with AI, a feature can work and pass its tests while violating the project's design. The instructions in the repository influence which result the agent will accept, so I have to develop and debug those instructions too.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture, design decisions and conclusions come from my work on Ze. The models helped organise the material and draft the text.*

## Define the whole objective

Suppose I ask an agent to add a plugin to Ze. It could add a branch to a central switch in the core, calling the new plugin directly. The feature could produce the expected result and pass a test of that result.

Each later plugin would then require another core change. That defeats the independence I want for plugin authors, who should be able to extend Ze without editing the code which finds and runs their plugins.

In Ze's [registration pattern](https://github.com/ze-software/ze/blob/main/ai/patterns/registration.md), each plugin adds its own definition to a shared registry, a table of available plugins and the functions needed to run them. The core looks up that definition by name, so the lookup code stays the same when a plugin is added. Giving the agent this explanation and an approved example makes the architectural constraint part of the task before it chooses an implementation.

Independence is part of the objective, alongside handling invalid input and avoiding unnecessary resource use. Those expectations are familiar to me from developing a routing daemon, but leaving them out gives the agent an incomplete account of what it has to achieve. The test in this example would leave those other requirements unexamined.

## The repository supplies the objectives

From the start, I wanted to design a system for developing Ze alongside the software itself. The repository would hold the requirements and explain how to check them, so an agent could recognise incomplete or incorrect work before presenting it as finished. The detailed procedures have evolved as we observed where agents went wrong, including where they followed my instructions and still produced the wrong result.

Writing these instructions is a form of metaprogramming: I am writing rules for how another program develops and evaluates Ze. The application around the model, usually called its harness, supplies the tools to edit files and run commands. The repository supplies the objectives and the means of judging the result for this particular project. Getting that judgement wrong can teach an agent to satisfy a check while leaving the feature broken.

Ze has many such constraints, so putting every rule into every prompt would surround the relevant guidance with pages unrelated to the task. The [root instructions](https://github.com/ze-software/ze/blob/main/AGENTS.md), maintained in [`ai/INSTRUCTIONS.md`](https://github.com/ze-software/ze/blob/main/ai/INSTRUCTIONS.md), explain how to find the applicable rules and design documents. The [task index](https://github.com/ze-software/ze/blob/main/ai/INDEX.md) then gives the session a starting point for the part of Ze it is changing.

The links between code and design are the subject of [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/). Once the agent has found the applicable requirements, it still needs to establish whether its implementation meets them.

## Choose evidence for the requested behaviour

Suppose I ask for a setting to take effect when Ze reloads its configuration. A unit test can confirm that a helper parses the new value correctly while leaving unchecked whether the running daemon ever applies it. If the agent treats that result as completion, it can stop before the requested feature works.

The instructions therefore have to say how far through the system the test must reach and under which conditions the result must hold. A parser needs checks of malformed input as well as valid packets. A subsystem that works in one operation may fail when several run concurrently, and Ze can agree with its own test peer while another implementation rejects its messages. Fuzzing, race detection and interoperability tests address different parts of those risks.

Ze's [testing rules](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) and [functional-test guide](https://github.com/ze-software/ze/blob/main/docs/functional-tests.md) connect the kind of change to the evidence required. A change to BGP encoding calls for an observation of the transmitted bytes. A command must be exercised through the CLI, and a reload must be checked through the daemon using the new configuration. Stating those requirements before implementation gives the agent a way to recognise that a convenient passing unit test has left the requested behaviour unchecked.

A routing daemon that produces correct results can still become unusable if it consumes excessive CPU time or memory. Too much processing per route can prevent it from keeping up with incoming updates, while excessive memory use can exhaust the host. A small test returning the right answer can leave both operational risks unchecked.

Those operational constraints belong in the instructions too. The allocation checks described in [How Ze reuses memory for BGP UPDATEs](../how-ze-manages-memory/) help detect unnecessary allocations in operations we have chosen to optimise. Benchmarks can show whether an operation has become more expensive, while tests of the running application are needed to establish how it behaves under load. The agent needs criteria for judging resource use alongside correct routing behaviour before it can decide that the change is complete.

I rely on tests to check much of the implementation detail, which means I also need a process for judging the tests. Ze requires another agent to review the code and tests against the agreed specification, and defects found during review must be corrected and reviewed again. That gives my trust in the result a basis beyond the author's assessment of its own work. Both agents can still share a misunderstanding, so I retain responsibility for the architecture and for deciding what the work must achieve, as I discussed in [AI slop is human slop](../ai-slop-is-the-wrong-test/).

## Give the agent useful feedback

A rule can be clear in a document and become misleading when a check reports it. Plugin registration provided an example. Ze's [import generator](https://github.com/ze-software/ze/blob/main/internal/le/plugin/imports/pluginimports.go) discovers plugin packages by their `register.go` files, so moving the registration to another filename can leave a plugin out of the generated imports. A check after an edit can catch that mistake before later code depends on the missing plugin.

The [Go edit check](https://github.com/ze-software/ze/blob/main/internal/le/hookruntime/writeedit.go) flags registration calls inside Go's `init()` function when they appear outside a registration file. Its message says `BLOCKED: Implicit behavior in init()`, although the approved pattern uses the same function inside `register.go`. An earlier version explained where the registration belonged and linked to the pattern, but that explanation was lost. The current message still omits it, so an agent could remove the registration mechanism while trying to resolve a problem with its location.

The intended correction is to move the registration into `register.go`, where the generator can discover the plugin. The refusal needs to say that and link to an example, because its wording becomes an instruction for the agent's next edit. This remains a weakness in the check: it can undo the guidance we supplied before implementation began.

## Establish that the test detects the failure

Even a test of the running daemon can pass for the wrong reason. In August 2026, three route redistribution scenarios continued to pass after late-join replay had been disabled. Replay is supposed to send existing routes to a peer which joins later, but the route reached the peer through another path in these scenarios. The expected route was present even though the feature under test was broken.

Deliberately disabling replay exposed a weakness that another run of the unchanged test would not have found. A Claude session began doing this without being asked, which was a welcome use of its time. Rebuilding the program with the fault and running the test showed that the setup or observations needed correcting before we could rely on the test to protect replay. A failure under that deliberate break would demonstrate its ability to catch that particular mistake, while leaving other possible faults to be checked separately.

Tests can be misleading in another way when they reproduce the intended implementation themselves. Also that summer, three BGP reactor tests maintained their own record of which address families had been sent an End-of-RIB marker, although the production code had no such record. These markers tell a peer that the initial route exchange for an address family is complete. An agent reviewing the tests nearly reported a conformance violation because it was analysing that test-only bookkeeping as though it were daemon code.

We tried an automated detector for these copies, but it also flagged ordinary table-driven test data. Those false alarms made it unreliable, so we replaced it with a review instruction: find the production function and confirm that the test calls it. The meaning of the assertions still requires judgement, but the reviewer begins with the implementation that runs in Ze.

[The proof is the expensive part](../the-proof-is-the-expensive-part/) follows this problem into RFC testing. There, a requirement identifier can help an agent find the relevant test, but the agent still has to establish that the test observes the application behaviour the requirement describes.

## Protect the evidence from the next edit

Once a test detects a defect, the next edit has to preserve what the test was checking. An agent can instead change the expected result to match the defective code, after which the test passes and the defect remains. Humans do this too, usually more slowly and with better excuses.

Ze's test-edit hook tries to detect changes that reduce what a test proves. Legitimate changes to the software can also require a test to change, so the [process](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) requires an independent agent to judge changes that could weaken a test. Changes to RFC-tagged tests also require that independent judgement. The agent making the change cannot approve its own explanation.

Recognising that weakening automatically has been difficult. An early hook counted non-comment lines in scenarios, so replacing several sleeps with one wait condition triggered a refusal even though the assertions remained. Meanwhile, changing an expected value in place could pass unnoticed. The line count was a poor substitute for understanding what the test checked.

The [August 2026 journal](https://github.com/ze-software/ze/blob/main/plan/journal/hook-existing-patterns-false-positive.md) records repeated false alarms, and the [audit of exemption comments](https://github.com/ze-software/ze/blob/main/docs/architecture/testing/TEST-RELAX-AUDIT.md) describes how hundreds of routine exemptions obscured genuine reductions in coverage. Moving the justifications into a ledger made them easier to collect, but did not make the detector accurate. The current process records the independent judge's approval with the commit; storing a justification and judging whether it is valid are separate jobs.

Repeated false alarms can teach the agent to request an exemption as a normal part of editing. We then lose the pause in which it was meant to reconsider whether it had weakened the test, and end up teaching it how to get past the check instead.

## Treat the rules as part of development

The same reasoning applies to when we run checks. A quick check after an edit can prevent later code from depending on a mistake. Running every expensive test at that point would use time we could spend on implementation. The instructions must help the agent select evidence appropriate to the change, and the checks must provide enough information for it to act on the result.

I accept the maintenance cost when a check helps the agent find and correct a mistake. The cost includes updating links when code moves and investigating false alarms, as well as running the checks themselves. Sometimes improving the process means correcting an error message. Sometimes, as with the detector for tests that copied production logic, it means removing a check and making the review instruction more precise.

A smaller project can build this gradually. A runnable checkout and a short index leading to the relevant design and an approved example give the agent somewhere useful to start. The project can define the evidence a change requires before implementation begins, and arrange for another session to review whether the result meets the objective. A mechanical check earns its place when it can recognise a recurring mistake reliably and explain the correction.

A correction made only in a conversation is easy to lose when the next session begins. Recording the reason and including it in the review instructions lets later sessions use it too. That is why I develop the process alongside Ze: an improvement should help with the next change as well as the one that exposed the problem.

This account began in summer 2026, and the models keep changing. A new model may need fewer instructions, or use guidance that an earlier one ignored. I have to reconsider which rules still help instead of preserving every restriction because it once solved a problem.

Claude has been contributing vocabulary as well as code. I called valid and invalid cases positive and negative tests; apparently we now have two polarities. Important decisions keep becoming load-bearing, and I expect the larger ones to become load-banging if this continues.

*Originally written: 9 August 2026. Last updated: 13 November 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-repository-is-the-ai-harness.md).*
