---
title: AI slop is the wrong test
date: 2026-08-03
author: Thomas Mangin
description: AI often writes better code than I would, and considerably more tests per feature. I have little patience for calling it all slop when human-written software has given us so much to complain about.

deck: I am using AI to build a router, and I remain responsible for what gets released. I also expect our tools to improve, as they have before.

image: assets/blog/ai-slop-is-the-wrong-test.svg
image-dark: assets/blog/ai-slop-is-the-wrong-test-dark.svg
image-alt: Fast generated output enters a narrowing evidence path through protocol checks, fuzzing, integration and benchmarking before it is accepted or rejected.

---

Ze is an AI-written network operating system, so I have a fairly direct interest in the argument about "AI slop". I decide the architecture and the tradeoffs, including what the code must never break, and Claude turns that into implementation. I also decide what gets rejected, which occupies rather more time than I had hoped.

I use AI for these articles too. I am lazy, my time is limited, and help putting my thoughts into words leaves me more time for the decisions and corrections. What goes out under my name remains my responsibility.

There is plenty of slop around. Code gets merged because it compiled, or because a demo worked once, without anyone looking much further. AI makes this much cheaper, but people were quite capable of doing it before.

Quite often, AI writes better code than I could have written myself. It is more meticulous, and writes far more tests per feature than I ever did. Dismissing all of that because the tools also fail seems a strange way to judge a technology, especially in an industry which has spent so long improving its tools.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## The compiler was not always good enough

In July 1983, Ed Post's [Real Programmers Don't Use Pascal](https://www.pbm.com/~lindahl/real.programmers.html) made fun of programmers for whom FORTRAN and assembly were evidence of competence. It is still amusing, although the assembly programmer sometimes had a perfectly good reason to object. Knowing the processor and choosing its instructions carefully could produce a faster inner loop than the compiler managed.

Michael Abrash's [Graphics Programming Black Book](https://www.jagregory.com/abrash-black-book/) explains how much could be gained from that knowledge, and the released [Quake source](https://github.com/id-Software/Quake/tree/master/WinQuake) has hand-written x86 assembly alongside its C rendering code. These programmers understood what the machine was doing and used that understanding to make something possible. I have no argument with that.

But compilers improved, and the programs we wanted to write grew. Saving instructions in one loop had to be weighed against maintaining the rest of the program and moving it to another machine. At some point, insisting on writing everything in assembly meant spending time on something the compiler could do well enough, while the rest of the project waited. Being right about a compiler's failings did not make that a good choice forever.

[FFmpeg's x86 codec code](https://github.com/FFmpeg/FFmpeg/tree/master/libavcodec/x86) still contains extensive assembly implementations. There are places where the gain justifies the effort, and recognising them is part of the job. I would not write a control plane in assembly, however much I might admire someone else's inner loop.

A model can misunderstand the request or invent an API, which is quite different from a compiler translating a program under language rules. We have to account for those failures. I still see the same mistake in treating what today's tool does badly as a reason to dismiss what comes after it, or even the useful things it can already do.

## There are enough failures to keep us busy

The objections deserve more than an optimistic answer. Veracode's [2025 GenAI Code Security Report](https://www.veracode.com/blog/genai-code-security-report/) found that 45 per cent of generated samples failed its security tests across Java, Python, C# and JavaScript. That is the result of its evaluation, rather than a count of vulnerabilities in every AI-written project, but I would hardly use it to justify accepting generated code because it looks reasonable.

We can even be wrong about whether we are saving time. In [METR's early-2025 study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/), 16 experienced open-source developers took 19 per cent longer on tasks in repositories they knew when AI tools were allowed. They believed they had been faster. Being pleased with the tool was apparently not a very good measurement.

METR's [February 2026 update](https://metr.org/blog/2026-02-24-uplift-update/) says developers are likely to benefit more from newer tools, but the follow-up data gives only very weak evidence for the size of that improvement. Developers' reluctance to work without AI introduced selection effects, and concurrent agents made time spent harder to measure. I am interested in the next measurements rather than declaring the matter settled by the first ones.

In Ze, Claude can explore an implementation and revise it against the design, but it can also ignore a constraint it has just been given. The output still has to be checked. A BGP encoder has to produce the bytes the protocol requires and respect the surrounding code's ownership rules, even if the model has supplied a very convincing explanation of why its version is correct.

Calling a Go function and checking the result covers something different from starting Ze and speaking to it as a peer. I need both, and neither will discover a case nobody thought to exercise. The [quality model](https://ze-software.net/quality/) sets out what Ze requires: a parser failure should leave a regression test or a fuzz corpus entry, Linux behaviour needs an exercise on Linux, and a memory optimisation needs a repeatable benchmark with a stated allocation contract. These are requirements for acceptance; parts of Ze still have to meet them.

More tests per feature are welcome, but they need scrutiny too. A model can "fix" a failure by changing the expected result to agree with bad code, or by removing the assertion which caught it. Then the test suite is green and the bug is still there. [The proof is the expensive part](../the-proof-is-the-expensive-part/) goes into the trouble of tying tests to requirements and checking that they demonstrate what they say they do.

## I still have to understand the bug

I know BGP from operating it and implementing it, including being called late at night to fix my own code. So when Claude proposes a shortcut in route selection or a change to a peer's state, I have something to compare it with besides Claude's explanation. It would be rather unwise to rely on the same model to tell me whether its own mistake was a mistake.

Before Ze, I had not implemented IS-IS or OSPF. I can still bring my protocol debugging experience to them: understand how information reaches neighbouring routers, what belongs in the topology database and how the route calculation uses it. When something behaves strangely, the job is to reduce it to a packet or a state transition I can explain. Having done that for BGP is useful even when the next protocol is unfamiliar.

My maths is very rusty, but I understand Dijkstra's algorithm and enough set theory to reason about reachability and shortest paths. I also know enough about packets and sockets to recognise when the kernel might be involved. I still use Wireshark to help decode TCP; using a tool does not remove the need to notice when what it shows contradicts what I expected.

That is the experience I can spend differently with Claude. I can have it try an implementation while I concentrate on the design, then stop it when the result is wrong and go back to the RFC. But I must still be able to debug the problem and build a reproduction of the rule it broke. If I cannot do that without the model, I cannot safely ask it to write the code.

Reading the diff is only part of this. A change can look reasonable and fail across a peer connection or when the router reaches capacity, and reading the same lines more carefully will not make that interaction appear on the screen. Review has to establish which test would expose the failure, and whether the model has made a design decision it was never authorised to make. This takes time, even when writing the implementation took very little.

## Human-written software has hardly been a guarantee

What annoys me in this debate is the implied quality of the software we had before AI arrived. We have spent decades producing programs which are slow, wasteful and difficult to understand, with a human responsible for every line. I have little patience for "Clean Code" used as a substitute for taste, or for a high-level language used as permission to ignore what the program compiles to. We managed all of this without a language model.

Of course, generating code faster with the same weak checks will give us more bad software. I expect plenty of projects to do exactly that, and deserve the criticism which follows. It is also possible to improve those checks and give the generator enough information to do a better job. I am much more interested in doing that than in defending human typing as a quality control process.

There is no shortage of software we would like to have, or existing software that needs attention. In my view, we do not have enough good programmers to write and maintain it all. A tool which lets an experienced engineer attempt more is worth having, and Ze has given me good reasons to keep using AI. Better models will help; so will the tools and practices we are still learning to build around them.

I expect considerable progress here, and learning how to use these tools now seems a better use of my time than waiting for everyone to agree they are useful. That optimism does not change who is responsible when something goes wrong. Claude cannot answer for an outage, a route leak or a security hole, and users should never have to excuse one because of how Ze was written.

I expected Ze to be finished by now. It is not ready, and the issues we keep finding have taken longer to fix than I expected. The repository is open, with the tests and design notes available for scrutiny, but I would rather delay Ze than release it with problems I already know about. When it is ready, I hope you will like it.

*Last updated: 11 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/ai-slop-is-the-wrong-test.md).*
