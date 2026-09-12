# The repository is half the AI harness

*2026-08-09 by Thomas Mangin*

The agent can search and edit. It still has to discover why the project is built this way, preferably before undoing it.

![General harness abilities and repository-specific meaning converge on an agent change; checks return failures to the rule and example that guide the next revision.](../../assets/blog/the-repository-is-the-ai-harness.svg)

One of the more frustrating parts of building Ze with AI is explaining a decision, getting an implementation which follows it, then having to explain it again to the next session. The code survives, but without the reasoning behind it an arriving agent can find a perfectly plausible way to undo the choice.

Ze is a network operating system spread over 780+ Go packages. A harness gives an agent the ability to search those packages and edit them, while the repository has to explain why they are built that way. Putting every explanation in the opening prompt would leave most of it unrelated to the task, so the agent needs to learn where to look and when to look there.

In [AI slop is the wrong test](../ai-slop-is-the-wrong-test/), I argued that I remain responsible for the generated code. Giving the next session useful guidance is part of that responsibility, and so is deciding what happens when it ignores the guidance. A check which stops the wrong change can help teach the project, but a badly chosen refusal can teach the wrong lesson.

*This article was co-authored with Claude and revised with OpenAI Codex. The architecture, design decisions and conclusions come from my work on Ze. The models helped organise the material and draft the text.*

## Giving the guidance somewhere to live

The instructions an agent meets at the root of Ze are generated from [`ai/INSTRUCTIONS.md`](https://github.com/ze-software/ze/blob/main/ai/INSTRUCTIONS.md). The canonical skills and rules live under `ai/`, and the [mirror generator](https://github.com/ze-software/ze/blob/main/internal/le/ai/ai.go) produces `CLAUDE.md`, `AGENTS.md` and the tool-specific skill copies. I can change the guidance in its own place rather than maintain a separate account for each tool.

Those instructions lead into [`ai/INDEX.md`](https://github.com/ze-software/ze/blob/main/ai/INDEX.md), where the task determines what to read next. Adding a plugin leads to the plugin pattern and its rules, while changing configuration leads elsewhere. The package map supplies short descriptions, so the agent can choose where to look without touring hundreds of directories first.

Once it reaches the source, a `// Design:` header leads to the explanation, and `// Detail:`, `// Overview:` and `// Related:` comments identify neighbouring files. Documents cite producing source through `<!-- source: ... -->` anchors. The generated `ai/CODE-TO-DOCS.md` index finds the documents which cite a file, while `ai/DOCS-TO-CODE.md` finds the source files which name a design.

The [navigation guide](https://github.com/ze-software/ze/blob/main/docs/contributing/navigating-the-code.md) explains when each route is useful and when a symbol lookup is enough. The agent is expected to consult the relevant explanation before changing the implementation, then keep that explanation current. Without that habit, the next session inherits a document which describes what the code used to do.

The source itself also has to be readable once the agent gets there. I prefer a file to deal with one concern, so understanding it does not require absorbing several unrelated jobs at once. Cutting a coherent implementation into arbitrary small pieces has the opposite effect: the reader has more files to visit and still has to understand the whole thing. Humans have the same problem, with rather more time between forgetting a decision and rediscovering it.

## A plausible change can be the wrong design

Plugin registration shows why finding the earlier decision changes an implementation. A central dispatch switch is a reasonable solution when the complete set of commands is known, and an agent can write a perfectly serviceable one. I chose registries for Ze because plugins, including third-party extensions, should remain separate from the core. Adding another branch in core code for every new plugin would defeat that choice.

The [registration pattern](https://github.com/ze-software/ze/blob/main/ai/patterns/registration.md) explains this, and the [Graceful Restart plugin](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/gr/register.go) supplies an example. Its `register.go` names `bgp-gr`, declares its capabilities and supplies the engine entry point. Registration happens through `registry.Register` in `init()`.

There is a reason for the filename too. The [import generator](https://github.com/ze-software/ze/blob/main/internal/le/plugin/imports/pluginimports.go) looks for `register.go` under the plugin roots and derives which packages the binary must load. Putting the same Go code in another file can look equivalent to somebody reading the package, but discovery also depends on where it lives.

I wanted that mistake caught during the edit, before the agent built more code on top of it. [`.claude/settings.json`](https://github.com/ze-software/ze/blob/main/.claude/settings.json) wires Write and Edit events to the native hooks in `le`. The [Go edit hook](https://github.com/ze-software/ze/blob/main/internal/le/hookruntime/writeedit.go) searches for registration-like text shortly after an `init()` declaration and exempts filenames beginning with `register`, so it can refuse the edit while there is still only one file to repair.

Unfortunately, its current message is `BLOCKED: Implicit behavior in init()`. We use `init()` in the approved pattern! The earlier hook included an explanation and a link which this message has lost, and an obedient agent could read the refusal as an instruction to remove the very mechanism the project requires. The correction is to put the registration in `register.go`; the message ought to say that.

A hook's answer becomes part of the agent's next prompt, so the quality of that answer affects what it writes next. Naming the file and pointing to the accepted example would give it a way to preserve the design while correcting the edit. With a bare prohibition it has to infer what the check wants, even though the repository already contains the answer.

## A door can pass its test

Choosing a test is another decision the agent needs help with before it starts writing. A model can produce a unit test, turn it green and report completion for a change whose behaviour only exists in the running daemon. Once it has chosen to observe so little of the system, more assertions in the same test may do nothing to close the gap.

I think about testing along two axes. One is how far through the system a test reaches: a function, connected components, the running application, then an exchange with somebody else's implementation. A parser can return the expected values while the daemon sends the wrong bytes, and Ze can agree with its own test peer while another BGP implementation rejects the exchange.

A door handle can operate its latch on a bench, and an assembled door can open and close as expected. Neither result establishes that the building is secure if the door has been fitted beside a large hole in the wall. Using somebody else's frame or lock adds assumptions the original tests did not exercise either. I find this a more useful way to think about a test suite than counting how many tests it contains.

The other axis is what we subject it to. Fuzzing supplies inputs the author did not think of, and race detection looks for conflicting concurrent access. Mutation testing deliberately breaks the code to see whether the tests notice; allocation checks and benchmarks expose resource costs that ordinary examples can miss. A routing daemon consuming uncontrolled CPU or memory can fail operationally, and the same behaviour may become a denial-of-service weakness.

These axes cross, so choosing how far a test reaches still leaves the conditions under which it runs to decide. The parser needs malformed inputs as well as correct output, while a subsystem may need its ordinary scenario exercised during concurrent activity.

Ze's [testing rules](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) and [functional-test guide](https://github.com/ze-software/ze/blob/main/docs/functional-tests.md) turn that into guidance for the task. A wire change calls for observing bytes, a CLI change for running the command, and a reload for an observation through the daemon. The choice follows from what the change could break, rather than from whichever test is easiest to write.

Several sessions may share the checkout, so new scenarios are developed in `test/draft/` before they enter a live suite. Otherwise one agent's unfinished fixture can interrupt another's verification. The draft eventually has to be promoted or deleted; leaving it there gives the next session an experiment with no explanation of what happened to it.

## The green result which proved too little

Even a test at the right level can observe the wrong thing. In August, three redistribution scenarios stayed green after their late-join replay had been disabled. The route still reached the peer through another path, so the scenarios saw the result they expected without establishing whether replay had happened.

Trying a behavioural test against a deliberate break is worth the extra rebuild and run for this reason. A Claude session began doing it without being asked, which was a welcome use of the model's time. If disabling the intended path leaves the scenario green, there is a fixture to repair before its verdict can tell us whether the implementation is right.

Some tests need less investigation to reveal why their green result proves too little. The [test-sensitivity scanner](https://github.com/ze-software/ze/blob/main/internal/le/testsensitivity/testsensitivity.go) finds Go tests with no reachable failure call, and test files whose build tags no native test action can supply. The test-health report puts these findings beside the inventory, where a test which cannot fail or never gets run would otherwise add one to an impressive total.

The BGP reactor tests produced a different surprise. Three maintained their own map of which address families had been sent an End-of-RIB, although production had no such map. A session reading them nearly reported a conformance violation in code which did not exist. The test had become an alternative implementation, and the model was reasoning about that one.

A detector was tried, but ordinary table-driven fixtures caused too many false alarms to keep it. The replacement was a reading rule: find the production function and confirm that the test calls it. Reading the assertions still requires judgement, but the reviewer at least starts by establishing which implementation is under discussion.

Recording these failures gives later sessions a reason to look beyond the test count and the green result which accepted them. [The proof is the expensive part](../the-proof-is-the-expensive-part/) follows the same problem into RFC compliance, where naming a requirement on a test can be mistaken for observing the behaviour it requires.

## When the agent changes the test

There is also a way for the agent to lose useful evidence while trying to fix a failure. Faced with a red test, it can change the expected result to match its implementation, and everything then agrees. Humans do this too, usually more slowly and with better excuses, so the test-edit hook tries to make a weakening visible before the rest of the checks start agreeing with it.

Some tests do need to be retired or weakened as the system changes. The [current process](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md) records the justification in a per-session ledger, and RFC-tagged tests require a separate record of the user's approval because they support public statements about Ze. The agent's explanation of why it wants to change one of those tests cannot authorise its own change.

Getting the hook to recognise a weakening has been less straightforward. An early version counted non-comment lines in scenarios, so replacing several sleeps with one wait condition looked like lost coverage. Changing an assertion in place could preserve the count and pass. We had managed to penalise a timing repair while letting the change we were worried about through.

The [10 August journal entry](https://github.com/ze-software/ze/blob/main/plan/journal/hook-existing-patterns-false-positive.md) recorded 751 `test-relax:` tokens across 466 files, including 362 in scenario files excusing timing refactors. Most were receipts for getting past the hook. Sorting genuine concessions out of that collection required an audit which should never have been necessary, and two days later the revised counter complained about combining peer processes while retaining the assertions.

Those inline markers belong to the old workflow. Moving approval into a ledger improves the record, but it does not teach a structural counter what an assertion means. The native hook passes proposed edits through the test-weakening checker, and an expected value can still change without any count changing. Repeated false refusals train the agent to produce the escape text as part of its normal edit, precisely when I need it to stop and explain what is being given up.

## Paying for the checks as well

The false positives cost time on both sides of a check. Somebody has to diagnose each refusal, then maintain the mechanism which produced it, and the same is true of the guidance around the code. Generated indexes replace tables somebody would otherwise maintain, but a moved package can still break a route into the design, and the generator needs looking after when the arrangement changes.

The expensive checks also cannot run after every edit. Focused feedback belongs close to the edit, while wider verification needs its own place in the process. The [commit helper](https://github.com/ze-software/ze/blob/main/docs/contributing/committing.md) now records missing verification as debt on a local commit and refuses an authorised push while debt remains open. Earlier versions of this article described a policy which required matching successful verification before a normal commit; that has changed, and the weakening-approval ledger serves a different purpose from this outstanding verification debt.

A bad rule can be applied consistently across many files before anyone notices, and that consistency can make the mistake look deliberate. Sometimes the useful change is to remove the check, as with the detector for tests which modelled production. Adding another exception would have left every later edit paying for a detector which had already proved too unreliable.

I accept the maintenance when it lets the agent recognise and correct a mistake without waiting for me. My time can then go into the design and into failures the checks do not understand, including failures in the checks themselves. A smaller project need not take on the entire bill: a runnable folder with predictable places for configuration and fixtures, plus a task index leading to one approved example, already gives an arriving agent somewhere useful to start.

A recurring mistake can justify a narrow check once it can be recognised reliably, and a common verification command can run cheap checks before expensive ones without spending a whole run on each preventable error. Copying Ze's entire arrangement would also copy the reasons it is expensive to maintain; the shared starting point I discuss in [AI coding has not had its Rails moment](../ai-coding-has-not-had-its-rails-moment/) would still leave each project to decide which checks it needs.

This account began in summer 2026, and new models keep changing which instructions are useful. A more capable model may need fewer of them, or may finally use guidance which an earlier one ignored. A rule which helped one model still has to justify the trouble it causes the next.

Claude has been contributing vocabulary as well as code. I called valid and invalid cases positive and negative tests; apparently we now have two polarities. Important decisions keep becoming load-bearing, and I expect the larger ones to become load-banging if this continues.

*Last updated: 11 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/the-repository-is-the-ai-harness.md).*
