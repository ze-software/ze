---
title: AI slop is human slop
date: 2026-08-03
author: Thomas Mangin
description: AI often writes better code than I would, and far more tests per feature. I judge that code by what it does and the evidence behind it, and remain responsible for what Ze releases.

deck: AI writes most of Ze's code. I remain responsible for what I release.

image: assets/blog/ai-slop-is-the-wrong-test.svg
image-dark: assets/blog/ai-slop-is-the-wrong-test-dark.svg
image-alt: Generated code is checked against protocol requirements, fuzz tests, integration tests and benchmarks.

---

Ze is an AI-written network operating system, and I remain responsible for what I release. I judge its code by the behaviour we can establish, including what our tests still leave unchecked.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## Why I keep using AI

I decide Ze's architecture and the trade-offs I am prepared to accept, and coding agents write most of the code. Quite often Claude writes better code than I could have written myself, and far more tests per feature than I ever did. That lets me attempt more than I could write alone, although its mistakes can take much longer to find and correct than I had hoped.

I use AI to help write these articles for much the same reason. Having help putting my thoughts into words leaves more time for developing Ze, but that saving is of little use to a reader who cannot follow the result. I am responsible for what goes out under my name, whether it is an explanation someone struggles to read or code that fails in their router.

Slop is code accepted without enough scrutiny: merged because it compiled, or trusted because the demo worked once. Human-written code can be slop too, and AI makes it much cheaper to produce. Calling all AI-written software "slop" before examining it substitutes a judgement about its origin for an investigation of the result.

## We have changed how we write software before

Judging programmers by their tools has a long history. In July 1983, Ed Post's [Real Programmers Don't Use Pascal](https://www.pbm.com/~lindahl/real.programmers.html) made fun of programmers who regarded FORTRAN and assembly as signs of competence. It is still amusing, although the preference for assembly sometimes had a practical basis: someone who knew the processor could choose instructions that made an inner loop faster than the compiler's version.

Michael Abrash's [Graphics Programming Black Book](https://www.jagregory.com/abrash-black-book/) explains how much could be gained from that knowledge, and the released [Quake source](https://github.com/id-Software/Quake/tree/master/WinQuake) includes hand-written x86 assembly alongside its C rendering code. The extra effort could produce a result the compiler could not deliver.

Improved compilers have made assembly a more specialised tool. When a video decoder has to keep up with playback on less powerful CPUs, spending time writing and optimising assembly for its most demanding routines can be worthwhile. I admire projects such as the [dav1d AV1 decoder](https://code.videolan.org/videolan/dav1d), which combines C with hand-written assembly tuned for particular processors.

AI requires a different kind of checking because a model can misunderstand a request or invent an API, while a compiler translates code according to the rules of a language. The comparison is about choosing a tool for the job and judging whether its benefits justify the work needed to use it safely. That judgement has to be revisited as the tools change.

## Human authorship never guaranteed quality

More capable tools give us choices, including the choice to produce wasteful software faster. We have spent decades writing programs which are slow and difficult to understand, with a human responsible for every line. I have little patience for applying "Clean Code" rules without considering whether they improve the program, or for using a high-level language as permission to ignore how much memory and processor time the result consumes.

A project which generates code faster while keeping weak checks can produce more bad software before anyone notices. A person typing the code would not fix the weakness in those checks. Clear requirements and a way to establish whether the program meets them give us a more useful basis for judging either author's output.

## The failures deserve scrutiny

Security testing shows how much can be missed when generated code is accepted on appearance. Veracode's [2025 GenAI Code Security Report](https://www.veracode.com/blog/genai-code-security-report/) found that 45 per cent of generated samples failed its security tests across Java, Python, C# and JavaScript. The figure describes those samples, so it cannot establish the vulnerability rate of every AI-written project. It does show why a plausible implementation needs testing before we trust it.

Even a claim about time saved needs more than the developer's impression. In [METR's early-2025 study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/), 16 experienced open-source developers took 19 per cent longer on tasks in repositories they knew when AI tools were allowed, although they believed they had been faster. Feeling productive while using a tool was a poor guide to how long the job took.

Repeating that measurement as the tools improve is harder than it might seem. METR's [February 2026 update](https://metr.org/blog/2026-02-24-uplift-update/) says developers are likely to benefit more from newer tools, but its follow-up data gives only very weak evidence for the size of that improvement. Developers were reluctant to work without AI, which affected who took part, and concurrent agents made time spent harder to measure. Those results do not give me a productivity figure for Ze, and my impression of getting more done cannot supply one either. I am interested in the next measurements because a comparison must account for changes in how we use the tools as well as changes to the tools themselves.

## What makes a change acceptable

Neither a broad study nor my experience of using Claude establishes that a particular change to Ze is correct. For a BGP encoder, we have to check the bytes it produces against what the protocol requires and check how it handles invalid input. An optimisation needs measurement too, or we could add complexity without reducing the cost of processing routes.

The saving has to preserve correct behaviour. Reusing a buffer can reduce allocations, but overwriting it while another reader still needs its contents can corrupt a route. In [How Ze reuses memory](../how-ze-manages-memory/), I explain how we decide when data can be shared and when it must be copied.

Checking the bytes returned by an encoder does not show that the running daemon sends them to another router. For that, we have to start the programs and observe their exchange. Ze's [quality model](https://ze-software.net/quality/) describes the checks different changes require. Some parts of Ze do not yet meet those requirements, and a passing run says nothing about a case the tests never exercise.

This is also why the number of tests Claude writes is a poor measure on its own. When a test fails, the model can change the expected result to match the bug or remove the assertion which caught it. The next run passes without restoring the required behaviour, so the tests need review against the requirements as well.

Ze's [development framework](../../project/ai-development/) makes that review part of a change. An agreed specification states the required behaviour and how it will be checked, then an agent implements it and another independently reviews the code and tests against that specification. For RFC requirements, deliberately breaking the implementation and observing the test fail gives us evidence that it detects that particular fault. Changes which could weaken a test also require independent judgement under the [testing rules](https://github.com/ze-software/ze/blob/main/ai/rules/testing.md).

I review the structure and overall implementation, and rely on this process to establish that the tests check the details correctly. [The proof is the expensive part](../the-proof-is-the-expensive-part/) explains the further work involved in connecting a protocol requirement to behaviour the running application actually demonstrates. A mistake shared by the author and reviewer can still survive, which is one reason my overview of the system remains necessary.

## Experience still matters

I know BGP from operating it and implementing it, including being called late at night to fix my own code. That experience helps me judge a proposed shortcut in route selection or a change to how a peer's connection is managed. I have something to compare with the agent's explanation, and a way to recognise when the investigation is overlooking part of the problem.

Before Ze, I had not implemented IS-IS or OSPF, so I have less direct experience to draw on. Debugging BGP still gives me a way to investigate: follow the information from the packet to the state the program keeps, and compare each change with the protocol requirements. In IS-IS and OSPF, that means checking that information reaches neighbouring routers and enters their topology databases before examining the route calculation which uses it.

My maths is very rusty, but I understand Dijkstra's algorithm and enough set theory to reason about reachability and shortest paths. That helps me judge the calculation once its inputs are correct, while knowledge of packets and sockets helps earlier in the investigation if the information never arrived. I use Wireshark to decode TCP, for example, but I still have to recognise when the packets it shows differ from what I expected.

Claude can explore alternatives and try an implementation while I concentrate on the design. I can delegate the reproduction and testing, but I must still be capable of explaining and investigating the failure independently of the model. When the evidence points beyond the function being changed, experience helps me recognise that the peer connection or the rest of the system needs investigating.

The review also has to identify decisions Claude made which I did not authorise. A change can pass its tests while abandoning an architectural constraint or narrowing what the feature does. Accepting that change would mean accepting a different objective, and that decision remains mine. Faster implementation does not give the model authority to decide which requirements are expendable.

## There is plenty worth building

The time spent reviewing and testing limits how quickly I can turn generated code into software I would release. Even after allowing for that time, I can attempt more with AI than I could write alone. There is no shortage of software we would like to have, or existing software that needs attention, and too few good programmers to maintain all of it. I expect the tools to improve considerably, and learning how to use and check them now seems a better use of my time than waiting for everyone to agree they are useful.

That expectation does not change what I owe someone who uses Ze. I remain responsible for an outage, route leak or security hole in software I release, regardless of how the code was written. I expected Ze to be finished by now, but the issues we keep finding have taken longer to fix than I expected. The repository is open, with the tests and design notes available for scrutiny, and I would rather delay Ze than release it with problems I already know about. When it is ready, I hope you will like it.

*Originally written: 3 August 2026. Last updated: 14 November 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/ai-slop-is-the-wrong-test.md).*
