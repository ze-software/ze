---
title: The repository is half the AI harness
date: 2026-08-09
author: Thomas Mangin
description: Ze's rules give AI agents explicit objectives and ways to check their work against the project's requirements, so they can recognise incomplete or incorrect implementations.

deck: Writing Ze with AI also means writing instructions for how the AI develops Ze. Those instructions must define success and help the agent recognise when its work falls short.

image: assets/blog/the-repository-is-the-ai-harness.svg
image-dark: assets/blog/the-repository-is-the-ai-harness-dark.svg
image-alt: Diagram showing an agent using coding tools and project instructions, then using check results to correct its change.

---

When I ask Claude to add a feature to Ze, much of what I expect can go unsaid. The code must fit the architecture, handle invalid input and avoid wasting memory, as well as produce the requested result. Those expectations are familiar to me from developing a routing daemon, but leaving them out of the request gives the agent an incomplete account of what it has to achieve.

I have therefore been recording the requirements and providing ways to check them. An agent implementing a command can find the design it must follow, then run checks which expose some of the ways its code could fail. When a check reports a problem, the agent can correct the implementation before presenting it as finished. This depends on the checks testing the qualities we care about and explaining their failures well enough to guide the next edit.

Writing these instructions is a form of metaprogramming: I am writing rules for how another program develops and evaluates Ze. The application around the model, usually called its harness, supplies the tools to edit files and run commands. The repository supplies the project requirements and checks that make those tools useful for this particular job. Much of my responsibility for the generated code, which I discussed in [AI slop is the wrong test](../ai-slop-is-the-wrong-test/), consists of developing this process and correcting it when it gives the agent the wrong guidance.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture, design decisions and conclusions come from my work on Ze. The models helped organise the material and draft the text.*

## Make the design part of the objective

A new plugin illustrates how an implementation can produce the requested result while missing part of the objective. We could add a branch to a central switch in Ze's core and use it to call the new plugin. The feature might work, but each later plugin would require another core change. That would defeat the separation which lets plugin authors extend Ze independently of the core.

With registration, each plugin supplies its own definition, including the function that starts it. The core looks up that definition in a shared registry, so the code for finding a plugin can stay the same as new ones are added. The [registration pattern](https://github.com/ze-software/ze/blob/main/ai/patterns/registration.md) explains this design and shows the code expected from a plugin.

Giving the agent that explanation before it starts makes the architectural constraint part of the task. A test that merely checks the new command's output would give it no reason to reject the central switch, whereas the documented goal of independent plugins explains why that implementation would be wrong for Ze.

Ze has many such constraints, so putting every rule into every prompt would surround the relevant guidance with pages unrelated to the task. Instead, [`ai/INDEX.md`](https://github.com/ze-software/ze/blob/main/ai/INDEX.md) directs the agent to the documents for the part of Ze it is changing. Root instructions explain how to find and use them, with [`ai/INSTRUCTIONS.md`](https://github.com/ze-software/ze/blob/main/ai/INSTRUCTIONS.md) serving as the source for the copies supplied to different AI tools.

The code also links to its design documents, so an investigation that starts with a function can recover the constraints behind it. [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/) develops this relationship between project knowledge and implementation. Here, it gives us a basis for validation: after identifying the requirements that apply, the agent can check its change against them.

## Give the agent useful feedback

For registration, one of those requirements concerns the filename. Each plugin's registration code belongs in `register.go`, because the [import generator](https://github.com/ze-software/ze/blob/main/internal/le/plugin/imports/pluginimports.go) searches for those files to find the packages to include. Moving the same code to another filename can leave the plugin out of the build.

A [check after Go edits](https://github.com/ze-software/ze/blob/main/internal/le/hookruntime/writeedit.go) tries to catch registration in the wrong file. Running it at that point gives the agent a chance to correct the mistake before building more code around it. But the usefulness of that feedback depends on the message explaining what went wrong.

The message was `BLOCKED: Implicit behavior in init()`, although the approved registration pattern itself uses Go's `init()` function. An earlier version had included an explanation and a link, but those had been lost. An agent following the remaining message could remove the registration mechanism when the required correction was to put it in `register.go`.

A check like this becomes part of the instructions the agent uses for its next edit. Naming the required file and linking to an example would help it make the correction we intended. An inaccurate refusal can direct it away from a design we have already explained elsewhere.

## Choose evidence for the requested behaviour

Other requirements cannot be checked by inspecting where the code is written. A configuration reload, for example, must change the behaviour of the running daemon. A unit test can confirm that a helper parses the new values correctly while leaving unchecked whether the daemon ever applies them. If the agent treats that result as completion, it can stop before the requested feature works.

Ze's [testing rules](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) and [functional-test guide](https://github.com/ze-software/ze/blob/main/docs/functional-tests.md) therefore connect the kind of change to the evidence required. A change to BGP encoding calls for an observation of the transmitted bytes. A command must be exercised through the CLI, and a reload must be checked through the daemon using the new configuration. These instructions explain what the test has to establish before the agent chooses how to write it.

We also have to specify the conditions under which the behaviour must hold. A parser tested only with valid input can fail on a malformed packet; a subsystem tested in isolation can fail when operations run concurrently. Fuzzing and race detection address some of those risks, while interoperability tests can expose a mistake shared by Ze and its own test peer.

Resource use is another part of the objective. A routing daemon that produces correct results but consumes excessive memory can still be unusable. The allocation checks described in [How Ze reuses memory for BGP UPDATEs](../how-ze-manages-memory/) give the agent evidence about the repeated operations we have chosen to optimise. That evidence complements the checks on their output, so an allocation saving cannot stand in for correct behaviour.

## Establish that the test detects the failure

Even a test of the running daemon can pass for the wrong reason. In August, three route redistribution scenarios continued to pass after late-join replay had been disabled. Replay is supposed to send existing routes to a peer which joins later, but the route reached the peer through another path in these scenarios. The expected route was present even though the feature under test was broken.

Deliberately disabling replay exposed a weakness which another run of the unchanged test would not have found. A Claude session began doing this without being asked, which was a welcome use of its time. The extra rebuild and test run showed that we needed to change the test setup or its observations before relying on it to protect replay.

Tests can be misleading in another way when they reproduce the intended implementation themselves. Three BGP reactor tests maintained their own record of which address families had been sent an End-of-RIB marker, although the production code had no such record. A session reading them nearly reported a conformance violation in the implementation invented by the tests.

We tried an automated detector for these copies, but it also flagged ordinary table-driven test data. Those false alarms made it unreliable, so we replaced it with a review instruction: find the production function and confirm that the test calls it. The meaning of the assertions still requires judgement, but the reviewer begins with the implementation that runs in Ze.

[The proof is the expensive part](../the-proof-is-the-expensive-part/) follows this problem into RFC testing. There, a requirement identifier can help an agent find the relevant test, but the agent still has to establish that the test observes the application behaviour the requirement describes.

## Keep a failed check useful

Once a test detects a defect, the next edit has to preserve the requirement it was checking. An agent can instead change the expected result to match the defective code, after which the test passes and the defect remains. Humans do this too, usually more slowly and with better excuses.

Ze's test-edit hook tries to detect changes that reduce coverage. Because legitimate changes to the software can also require a test to change, the [process](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) records the justification in a per-session ledger. Tests cited as evidence for RFC requirements also require a separate record of the user's approval. The agent cannot approve its own explanation for weakening evidence behind a public support claim.

Recognising that weakening automatically has been difficult. An early hook counted non-comment lines in scenarios, so replacing several sleeps with one wait condition triggered a refusal even though the assertions remained. Meanwhile, changing an expected value in place could pass unnoticed. The line count was a poor substitute for understanding what the test checked.

The [10 August journal entry](https://github.com/ze-software/ze/blob/main/plan/journal/hook-existing-patterns-false-positive.md) recorded hundreds of exemptions added to get harmless timing changes past this check. Those routine exemptions made genuine reductions in coverage harder to find. A separate ledger now keeps the justifications together, but it cannot make an inaccurate check reliable.

Repeated false alarms can teach the agent to request an exemption as a normal part of editing. We then lose the pause in which it was meant to reconsider whether it had weakened the test. The quality requirement has to remain the reason for the check, or we end up teaching the agent how to get past our tooling.

## Treat the rules as part of development

The same reasoning applies to when we run checks. A quick check after an edit can prevent later code from depending on a mistake. Running every expensive test at that point would use time we could spend on implementation. The instructions must help the agent select evidence appropriate to the change, and the checks must provide enough information for it to act on the result.

I accept the maintenance cost when this helps the agent identify incomplete or incorrect work and correct it. Sometimes that means improving an error message; sometimes, as with the detector for tests that copied production logic, it means removing a check and making the review instruction more precise. Adding another rule is useful only if it improves the development of Ze.

A smaller project can build this gradually. A runnable checkout and a short index leading to the relevant design and an approved example give the agent somewhere useful to start. When a mistake recurs, a check is worth adding if it can recognise that mistake reliably and explain the correction. The checks can then grow with the project's requirements, without importing every restriction we accumulated while building Ze.

A correction made only in a conversation is easy to lose when the next session begins. Recording the reason, linking it to the code and checking the resulting behaviour lets later sessions use it too. That is the lasting benefit of this metaprogramming: an improvement to the development instructions can improve more than the change we happen to be making today.

This account began in summer 2026, and the models keep changing. A new model may need fewer instructions, or use guidance that an earlier one ignored. I have to reconsider which rules still help instead of preserving every restriction because it once solved a problem.

Claude has been contributing vocabulary as well as code. I called valid and invalid cases positive and negative tests; apparently we now have two polarities. Important decisions keep becoming load-bearing, and I expect the larger ones to become load-banging if this continues.

*Last updated: 21 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-repository-is-the-ai-harness.md).*
