---
title: AI slop is the wrong test
date: 2026-08-03
author: Thomas Mangin
description: Ze is an AI-written network operating system. I remain responsible for what it ships, including the mistakes a model makes and the limits of the tests meant to catch them.

deck: I use AI to write Ze, but accepting its output requires expertise I can answer for when the software fails.

image: assets/blog/ai-slop-is-the-wrong-test.svg
image-dark: assets/blog/ai-slop-is-the-wrong-test-dark.svg
image-alt: Fast generated output enters a narrowing evidence path through protocol checks, fuzzing, integration and benchmarking before it is accepted or rejected.

---

Ze is an AI-written network operating system. I decide the architecture, the tradeoffs, what the code must never break and what gets rejected. Claude turns that into implementation.

I use AI to help write these articles too. I am lazy, and my time is limited, so I want to spend it on the judgement and the corrections, with help putting them into words. That does not transfer responsibility for either the software or the articles to the model.

"AI slop" tells me how someone thinks the code was produced, but gives me little to investigate. A claim that Ze mishandles a packet or that a test cannot catch the error gives me something I can answer. I want the argument about generated code to reach that level, because accepting code on the strength of a working demo is a failure whether a person typed it or a model produced it.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## The objections need their scope

In July 1983, [Real Programmers Don't Use Pascal](https://www.pbm.com/~lindahl/real.programmers.html) satirised programmers who treated FORTRAN and assembly as evidence of competence. I recognise the temptation to judge someone by their tools, though the comparison has limits. A compiler translates a program under language rules; a model can invent an API or misunderstand the requested behaviour, and the history of compilers cannot tell us how reliable AI coding will become.

There is evidence of the risks. Veracode's [2025 GenAI Code Security Report](https://www.veracode.com/blog/genai-code-security-report/) reported that 45 per cent of generated samples failed its security tests, across Java, Python, C# and JavaScript. That is a result from its evaluation, rather than an estimate of the vulnerable share of every AI-written codebase, and it is enough to reject the assumption that plausible output is safe output.

Productivity also needs measurement. In [METR's early-2025 study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/), 16 experienced open-source developers took 19 per cent longer on tasks in familiar repositories when AI tools were allowed, while believing they had been faster. METR now says that result no longer reflects current tools, but its [February 2026 update](https://metr.org/blog/2026-02-24-uplift-update/) also says the follow-up data gives only very weak evidence for the size of any improvement. Developers' reluctance to work without AI had introduced selection effects, and concurrent agent use made time spent harder to measure.

I cannot use either study to claim a productivity gain for Ze. They do give me a reason to distrust the feeling of progress when an agent has produced a large patch, and to examine how much of that patch survives review and correction.

## Ze assumes the generator is untrusted

I treat Claude as a generator which can follow precise constraints and will sometimes fail to follow them. It can explore alternatives and run tools, then revise the implementation against the design. I still have to decide whether the result is acceptable, including whether the tests themselves are asking the right thing.

For a BGP encoder, I want evidence that the bytes meet the protocol's requirements and that the implementation respects the ownership rules of the surrounding code. A test which calls a Go function checks a different boundary from one which starts Ze and talks to it as a peer. Both can pass while a case we did not think of remains broken.

Ze's public [quality model](https://ze-software.net/quality/) sets out which kinds of evidence belong at those different boundaries. A parser failure should leave a test or a fuzz corpus entry, and a claim about Linux behaviour needs an exercise on Linux. For a memory optimisation, I want a repeatable benchmark with a stated allocation contract. These are acceptance requirements; listing them is no claim that every part of Ze has met them.

A generated test can repeat the same misunderstanding as the generated implementation, so a green run needs scrutiny too. A model may remove the assertion which exposed a bug or change the expected result to match the code, and that leaves the failure in place. [The proof is the expensive part](../the-proof-is-the-expensive-part/) describes how Ze connects requirements to evidence and what it costs to check that the connection means anything.

## I have to be able to debug it

I know BGP because I have operated it, implemented it, broken it and been called to fix my own code late at night. That background matters when Claude proposes a route-selection shortcut or a peer-state transition. I can compare the suggestion with something beyond the explanation the model supplies.

I had not implemented IS-IS or OSPF before Ze, but I could bring my experience of protocol debugging to them. With link-state routing I have to reason about the topology database and the route calculation, as well as how information reaches neighbouring routers. If the implementation behaves strangely, I need to reduce the failure to a packet or a state transition that I can explain.

My maths is very rusty, but I understand Dijkstra's algorithm and enough set theory to reason about reachability and shortest paths. My networking knowledge is low-level enough to know when the packet, socket or kernel behaviour is the suspect, and my debugging skills come from maintaining my own code in production. I still use Wireshark to decode TCP when I need help, and I need to recognise when its answer contradicts what I expected.

When Claude is wrong, I must be able to stop it and read the RFC, then build a reproduction which demonstrates the rule it broke. If I cannot debug the problem without the model, I cannot safely ask the model to write the code. Writing down more instructions does not remove that limit.

## Accepting the code makes it mine

Reviewing a plausible diff is insufficient when the failure only appears across a peer connection or at capacity. I need to know which test would fail if the change were wrong, and which decisions the model took that it had no authority to take. Those questions apply to human-written code too, though generating a large change quickly makes it easy to leave them unanswered.

Claude cannot be responsible for an outage, a route leak or a security hole. The people who accept generated code and ship it are responsible, and I will not ask users to excuse a defect because of how Ze was written. My decision to use AI gives me the obligation to understand what I accept.

I expected Ze to be finished by now. It is not ready, and the issues we keep finding have taken longer to fix than I expected. The repository is open, with the tests and design notes available for scrutiny, but I would rather delay Ze than release it with problems I already know about.

*Last updated: 8 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/ai-slop-is-the-wrong-test.md).*
