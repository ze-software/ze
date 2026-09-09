---
title: The repository is half the AI harness
date: 2026-08-09
author: Thomas Mangin
description: Building Ze with AI has meant teaching the repository to explain its architecture, direct an agent towards the right evidence and retain what we learn when the checks are wrong.

deck: A correction in a conversation disappears with the session. I want the repository to retain the decision and help the next contributor apply it.

image: assets/blog/the-repository-is-the-ai-harness.svg
image-dark: assets/blog/the-repository-is-the-ai-harness-dark.svg
image-alt: General harness abilities and repository-specific meaning converge on an agent change; checks return failures to the rule and example that guide the next revision.

---

Building Ze with AI has made me spend a great deal of time explaining the project to something which will forget the explanation. Ze is a network operating system spread over {{ze:repo-go-packages}} Go packages, and an agent can open a file without knowing why it belongs in that package or which design decision a plausible change would undo.

The harness gives it the general abilities to search, edit and run programs. I have to supply the project-specific meaning, and repeating that meaning in each conversation is a poor way to retain it. A correction needs somewhere to live after the session ends, where the next agent can find it before repeating the mistake.

I argued in [AI slop is the wrong test](../ai-slop-is-the-wrong-test/) that I remain responsible for the generated code. That responsibility starts before the diff exists: I need an environment which helps the agent choose the right implementation and demands the right evidence when it is finished.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture, design decisions and conclusions come from my work on Ze. The models helped organise the material and draft the text.*

## Giving the next session a way in

I cannot put everything Ze knows into the opening prompt. Most of it would be irrelevant to the immediate task, and the relevant decision would still be buried somewhere inside it. I want the agent to arrive at the material through the job it is doing.

[`ai/INDEX.md`](https://github.com/ze-software/ze/blob/main/ai/INDEX.md) is the entrance. Adding a plugin leads to the plugin pattern and its rules; changing configuration leads elsewhere. The package map supplies short package descriptions, so the first search need not be a tour of hundreds of directories.

Once the agent reaches the source, `// Design:` headers point towards the explanation, and `// Detail:`, `// Overview:` and `// Related:` comments identify the neighbouring files. Documents cite their source through `<!-- source: ... -->` anchors. The generated `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` indexes answer the reverse lookups, from a source file to the documents which cite it, and from a design document to the files which name it.

These links let the agent recover why something was built this way instead of inferring the decision from whatever code it happens to open first. The [navigation guide](https://github.com/ze-software/ze/blob/main/docs/contributing/navigating-the-code.md) tells it when to use the indexes and when a symbol lookup is the better route. A link sitting in a file helps only if the contributor knows to follow it.

I also want source files to stay around one concern. A file which mixes unrelated jobs makes every later reader pay for all of them, while splitting a coherent implementation mechanically creates more navigation without reducing the reasoning. Humans benefit from the same organisation; we remember longer than an AI session, but we also forget why an old decision was made.

## An architectural decision has to survive an edit

Plugin registration shows why navigation alone is insufficient. A central dispatch switch is reasonable when the complete set of commands is known. I want Ze's plugins, including third-party extensions, to remain separate from the core, so requiring every new plugin to add a branch to that switch would undo the architecture.

Ze uses registries. The [registration pattern](https://github.com/ze-software/ze/blob/main/ai/patterns/registration.md) explains the arrangement, and an existing plugin shows what the explanation means in code. In the [Graceful Restart plugin's `register.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/register.go), the registration names `bgp-gr`, supplies its engine entry point and declares its capabilities. Its `init()` calls `registry.Register`.

The filename is part of the mechanism. The [import generator](https://github.com/ze-software/ze/blob/main/internal/le/plugin/imports/pluginimports.go), in `discoverPlugins`, scans the plugin roots for `register.go` and derives the packages the binary must load. The agent can therefore follow a task to the rule, see an implementation and understand why putting the same registration in an arbitrary file is insufficient.

I want a check to catch the recognisable mistake while the agent is making it. The [Go edit hook](https://github.com/ze-software/ze/blob/main/internal/le/hookruntime/writeedit.go), in `writeGoPatterns`, looks for registration-like calls near the start of an `init()` function and exempts filenames beginning with `register`. An early refusal can mean repairing one file before the agent builds several more on the assumption that the first one was acceptable.

The current diagnostic also shows how easy it is for the guidance and the check to disagree. It says `BLOCKED: Implicit behavior in init()`, although `init()` is part of the approved registration pattern. The earlier hook included an explanation and a link which the current message lacks. The useful correction is to move the registration into `register.go`, and the diagnostic should say so rather than invite the agent to remove a mechanism the project requires.

Whatever the hook prints becomes part of the next prompt. I want it to identify the affected file and the accepted implementation, with a link to the reasoning. A refusal which only names something forbidden encourages the model to find another spelling of the same mistake.

## Testing has two directions

The same problem appears when the agent chooses a test. A unit test is quick to write and quick to run, and a model can turn it green without ever reaching the behaviour an operator depends on. I need the repository to explain how much of the system a change has to exercise before implementation starts.

I think about that in two directions. The first is how far through the system the evidence reaches: a function, connected components, the running application, then an exchange with software written by somebody else. A BGP parser can return the right values while the daemon sends the wrong bytes, and Ze can agree with its own test peer while another implementation rejects the exchange.

A door is a useful way to picture that difference. A handle can operate its latch on the bench, and the assembled door can open and close perfectly, yet neither result establishes that the building is secure if the door was fitted beside a large hole in the wall. A frame or lock supplied by somebody else introduces another set of assumptions. Each test has to reach the level at which its claim lives.

The second direction is the conditions under which the behaviour survives. Fuzzing explores inputs the author did not think to supply; race detection looks for conflicting concurrent access. Mutation testing asks whether deliberately broken code can make the tests fail, while allocation checks and benchmarks can expose a resource cost which ordinary examples never reveal.

I include resource use among the safety concerns because a routing daemon which consumes uncontrolled CPU or memory can become an operational failure, and occasionally a denial-of-service weakness. These directions cross: the parser needs malformed inputs as well as correct output, and a subsystem may need its ordinary scenario exercised with concurrent activity. Counting unit tests says little about either decision.

Ze's [testing rules](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) and [functional-test guide](https://github.com/ze-software/ze/blob/main/docs/functional-tests.md) give the agent a route from the change to the required surface. Wire behaviour needs a byte-level observation, a CLI change needs the command to run, and a reload needs evidence through the running daemon. I do not want the author choosing whichever small test is easiest to satisfy and treating that convenience as an engineering judgement.

There is a practical consequence when several sessions share the checkout. Scenarios are developed in `test/draft/` before promotion into a live suite, so an unfinished fixture does not interrupt somebody else's verification. The draft must eventually be promoted or deleted. Leaving it there passes an unexplained half-finished experiment to the next session.

## Green tests taught me to distrust the choice

Choosing the right surface still leaves the possibility that a test observes the right result for the wrong reason. In August, three redistribution scenarios stayed green with their late-join replay disabled. The route reached the peer through another path, so the observation never established that replay had happened.

That is why I want a behavioural test tried against a deliberate break in the behaviour it claims to protect. A Claude session began doing this without being asked. Disabling the intended path, rebuilding and seeing the scenario fail can expose a fixture which needs redesigning before the implementation is worth trusting. It costs another run, and I have never regretted that expense.

Some weaker forms of false confidence can be checked mechanically. The [test-sensitivity scanner](https://github.com/ze-software/ze/blob/main/internal/le/testsensitivity/testsensitivity.go), in `Scan`, looks for Go tests with no reachable failure call and test files whose build tags no native test action can supply. The test-health report publishes those findings alongside the test inventory. I want that distinction visible because a larger suite can contain more tests which never run or cannot detect a failed expectation.

There was another failure in the BGP reactor tests. Three of them maintained their own map of which address families had been sent an End-of-RIB, although production had no such map. A session reading those tests nearly reported a conformance violation in code which did not exist.

A detector was tried, and ordinary table-driven fixtures made it too noisy to keep. The replacement was a reading habit: identify the production function and confirm that the test calls it. This cannot establish that every assertion is meaningful, but it stops the author treating a model of the implementation inside the test as evidence about the implementation itself.

Those incidents explain why I cannot leave testing policy at a required count or a green result. The repository has to retain the failure we learned from, so the next agent looks for the same mistake. [The proof is the expensive part](../the-proof-is-the-expensive-part/) follows an RFC claim through that harder relationship between a requirement and what its tests observe.

## The author must not be able to repair the evidence away

An AI which sees a failing test can change the test to agree with its implementation. The result is internally consistent and wrong. Humans do this too, usually more slowly and with better excuses, so I want a proposed weakening to be visible regardless of who wrote it.

The test-edit hook tries to catch that at the edit, before every later check starts agreeing with the changed expectation. The [current process](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) records legitimate weakenings in a per-session ledger. RFC-tagged tests need the user's approval recorded separately because those tests support public claims; the author's own explanation is insufficient authority to retire them.

That process has to distinguish a concession from an argument with a bad detector. An early hook counted non-comment lines in scenarios, so replacing several sleeps with one wait condition looked like lost coverage. Replacing assertions in place could leave the count unchanged and pass. We had built a check which penalised a timing repair while missing the change it was meant to prevent.

The [10 August journal entry](https://github.com/ze-software/ze/blob/main/plan/journal/hook-existing-patterns-false-positive.md) recorded 751 `test-relax:` tokens across 466 files, including 362 in scenario files excusing timing refactors. Most were receipts for getting past the hook, and separating genuine concessions from them took an audit I should not have needed. Two days later, the revised counter objected to combining peer processes while retaining the assertions, so the first repair had not removed the failure class.

Those inline markers belong to the older workflow, but changing their storage does not settle the semantic problem. The native hook reads proposed edits through the test-weakening checker, and an expected value changed in place can still preserve every structural count. I still have to judge what has been given up. Repeated false alarms make that harder because they teach the agent to supply the escape text as routine.

## What I am paying for

I would rather not pretend this machinery is free. Generated indexes remove hand-maintained tables, but they also create generated files to review and keep current. A renamed package or moved design leaves references which someone has to repair, and a check which exposes that drift has created an interruption even when it is correct.

Expensive verification cannot happen after every edit, so focused feedback and wider checks serve different points in the day. The [commit process](https://github.com/ze-software/ze/blob/main/docs/contributing/committing.md) now records missing verification as debt on a local commit and refuses an authorised push while debt remains open. That is different from the earlier policy described in this article, which required a matching successful verification before a normal commit.

False refusals spend maintainer time as well as machine time. When a rule is wrong, an obedient agent may already have copied it into many files, and the resulting consistency can make the mistake harder to see. Maintaining this environment includes changing or removing a check which costs more than the defects it catches.

I still think the trade is worth making. Without it, I have to recover the relevant decisions and inspect the repeated mistakes myself for every change, while the agent waits for another explanation it will forget. Automation moves some of that repetition onto the machine and leaves me with the decisions it cannot make reliably. It does not remove my responsibility for either side.

## A smaller beginning is possible

I would start a smaller project with a runnable folder and predictable places for its configuration and fixtures. An architecture map can explain what belongs in each directory, and a task-oriented index can send the contributor to one approved example. Those are useful before there is any elaborate verification machinery.

Source-to-design links become worth adding when the same decision keeps being rediscovered. A narrow check can follow a repeated mistake once there is a reliable way to recognise it, and its refusal needs the route back to the accepted pattern. The testing guidance then records which changes need an observation through the application and which need additional pressure from concurrency or hostile input.

A common verification command gives those checks an order, with cheap failures before expensive evidence and enough reporting to avoid discovering one preventable error per run. I would add each part because it saves a recurring explanation or catches a known mistake. Copying all of Ze would also copy the maintenance burden which produced it.

This account began in summer 2026, and the model releases keep moving the boundary. A more capable model may need less instruction, or may finally use richer guidance correctly. The case for shared conventions belongs in [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/); here I want the corrections to survive long enough for us to discover which ones are still necessary.

Claude has also developed a vocabulary while helping me build this. I called the valid and invalid cases positive and negative tests; Claude calls them the two polarities. Important decisions are repeatedly described as load-bearing, and I expect the larger ones to become load-banging if this continues. I normally remove the phrase from prose, but I am keeping the joke.

*Last updated: 9 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-repository-is-the-ai-harness.md).*
