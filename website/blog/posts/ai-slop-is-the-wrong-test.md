---
title: AI slop is the wrong test
date: 2026-08-03
author: Thomas Mangin
description: AI often writes better code than I would, and considerably more tests per feature. I have little patience for calling it all slop when human-written software has given us so much to complain about.

deck: I am using AI to build a router, and I remain responsible for what gets released. I also expect our tools to improve, as they have before.

image: assets/blog/ai-slop-is-the-wrong-test.svg
image-dark: assets/blog/ai-slop-is-the-wrong-test-dark.svg
image-alt: Generated code is checked against protocol requirements, fuzz tests, integration tests and benchmarks.

---

Ze is an AI-written network operating system. I decide the architecture and the trade-offs I am prepared to accept, and Claude writes the code. Quite often it writes better code than I could have written myself, and far more tests per feature than I ever did. That lets me attempt more than I could write alone, although the mistakes it makes can take much longer to find and correct than I had hoped.

I use AI to help write these articles for much the same reason. Help putting my thoughts into words leaves more time for developing Ze, but that saving is of little use to a reader who cannot follow the result. I am responsible for what goes out under my name, whether it is an explanation someone struggles to read or code that fails in their router.

There is plenty of poor AI-generated software, including code accepted because it compiled or because a demo worked once. Those are failures worth criticising because neither check establishes that the program will do its job. Calling all AI-written software "slop" avoids examining that distinction: it judges how the code was produced before considering whether the result is useful or reliable. My experience with Ze gives me reasons to keep using AI, and reasons to insist on better evidence than a convincing answer from Claude.

*This article was drafted and revised with OpenAI Codex. The ideas, experience and conclusions are mine.*

## We have changed how we write software before

Judging programmers by their tools has a long history. In July 1983, Ed Post's [Real Programmers Don't Use Pascal](https://www.pbm.com/~lindahl/real.programmers.html) made fun of programmers who regarded FORTRAN and assembly as signs of competence. It is still amusing, although the preference for assembly sometimes had a practical basis: someone who knew the processor could choose instructions that made an inner loop faster than the compiler's version.

Michael Abrash's [Graphics Programming Black Book](https://www.jagregory.com/abrash-black-book/) explains how much could be gained from that knowledge, and the released [Quake source](https://github.com/id-Software/Quake/tree/master/WinQuake) includes hand-written x86 assembly alongside its C rendering code. The extra effort could produce a result the compiler could not deliver.

As compilers improved, the benefit of doing all that work by hand became smaller. Saving a few instructions still took time away from the rest of the program, and assembly made moving it to another processor harder. Developers could spend that effort where the remaining gain justified it, as [FFmpeg's x86 codec code](https://github.com/FFmpeg/FFmpeg/tree/master/libavcodec/x86) still demonstrates. I admire what other programmers achieve with assembly without wanting to write a routing control plane in it.

AI requires a different kind of checking because a model can misunderstand a request or invent an API, while a compiler translates code according to the rules of a language. The comparison is useful for a narrower reason: our judgement of a tool has to change when its capabilities change. A limitation that once justified doing something by hand is a reason to measure the next version, rather than assume it can never be useful.

## Human authorship never guaranteed quality

More capable tools give us choices, including the choice to produce wasteful software faster. We have spent decades writing programs which are slow and difficult to understand, with a human responsible for every line. I have little patience for applying "Clean Code" rules without considering whether they improve the program, or for using a high-level language as permission to ignore how much memory and processor time the result consumes.

That history is why the implied quality of software before AI annoys me. If a project relies on the same weak checks while generating code faster, it can produce more bad software before anyone notices. A person typing the code would not fix the weakness in those checks. Clear requirements and a way to establish whether the program meets them give us a more useful basis for judging either author's output.

## We need to measure the results

Security testing shows how much can be missed when generated code is accepted on appearance. Veracode's [2025 GenAI Code Security Report](https://www.veracode.com/blog/genai-code-security-report/) found that 45 per cent of generated samples failed its security tests across Java, Python, C# and JavaScript. The figure describes those samples, so it cannot establish the vulnerability rate of every AI-written project. It does show why a plausible implementation needs testing before we trust it.

Even a claim about time saved needs more than the developer's impression. In [METR's early-2025 study](https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/), 16 experienced open-source developers took 19 per cent longer on tasks in repositories they knew when AI tools were allowed, although they believed they had been faster. Feeling productive while using a tool was a poor guide to how long the job took.

Repeating that measurement as the tools improve is harder than it might seem. METR's [February 2026 update](https://metr.org/blog/2026-02-24-uplift-update/) says developers are likely to benefit more from newer tools, but its follow-up data gives only very weak evidence for the size of that improvement. Developers were reluctant to work without AI, which affected who took part, and concurrent agents made time spent harder to measure. I am interested in the next measurements because a comparison must account for changes in how we use the tools as well as changes to the tools themselves.

Neither a broad study nor my experience of using Claude establishes that a particular change to Ze is correct. For a BGP encoder, we have to check the bytes it produces against what the protocol requires and check how it handles invalid input. An optimisation also needs a repeatable measurement of its memory use, or we could add complexity without reducing the cost of processing routes.

The saving has to preserve correct behaviour. If an optimisation reuses a buffer, we have to establish that no code still expects the previous contents when the buffer is overwritten. A lower allocation count would be a poor result if it came with corrupted routes. [How Ze reuses memory](../how-ze-manages-memory/) explains how the rules for sharing and retaining data belong to the same design as the intended memory saving.

The test has to observe the behaviour we are claiming. Calling an encoder function can check the bytes it returns, but it cannot show that the running daemon sends those bytes to another router. That requires starting the programs and observing their exchange. Ze's [quality model](https://ze-software.net/quality/) applies the same reasoning to other changes, including parser tests, Linux exercises for Linux behaviour and repeatable benchmarks for memory optimisations. Parts of Ze still have to meet those requirements, and a passing run says nothing about a case the tests never exercise.

This is also why the number of tests Claude writes is a poor measure on its own. When a test fails, the model can change the expected result to match the bug or remove the assertion which caught it. The next run passes without restoring the required behaviour, so reviewing the code also means examining what its tests establish. [The proof is the expensive part](../the-proof-is-the-expensive-part/) describes how we connect those tests to protocol requirements and check whether they would detect an incorrect implementation.

## I still have to understand the bug

To judge those tests, I need enough knowledge of the protocol to notice when the expected result is wrong. I know BGP from operating it and implementing it, including being called late at night to fix my own code. When Claude proposes a shortcut in route selection or changes how a peer's connection is managed, I can compare that proposal with my own understanding. Without it, I would be relying on Claude to identify mistakes in its own answer.

Before Ze, I had not implemented IS-IS or OSPF, so I have less direct experience to draw on. Debugging BGP still gives me a way to investigate: follow the information from the packet to the state the program keeps, and compare each change with the protocol requirements. In IS-IS and OSPF, that means checking that information reaches neighbouring routers and enters their topology databases before examining the route calculation which uses it.

My maths is very rusty, but I understand Dijkstra's algorithm and enough set theory to reason about reachability and shortest paths. That helps me judge the calculation once its inputs are correct, while knowledge of packets and sockets helps earlier in the investigation if the information never arrived. I use Wireshark to decode TCP, for example, but I still have to recognise when the packets it shows differ from what I expected.

Claude can try an implementation while I concentrate on the design, and we can return to the RFC when something is unclear. To accept a proposed fix, though, I must be able to explain and reproduce the failure without relying on the model's diagnosis. Some failures appear only when Ze communicates with another router or runs out of capacity, so reading the code has to be followed by tests which expose the behaviour.

The review also has to identify decisions Claude made which I did not authorise. A change can pass its tests while abandoning an architectural constraint or narrowing what the feature does. Accepting that change would mean accepting a different objective, and that decision remains mine. Faster implementation does not give the model authority to decide which requirements are expendable.

## There is plenty worth building

The time spent reviewing and testing limits how quickly I can turn generated code into software I would release. Even after allowing for that time, I can attempt more with AI than I could write alone. There is no shortage of software we would like to have, or existing software that needs attention, and too few good programmers to maintain all of it. I expect the tools to improve considerably, and learning how to use and check them now seems a better use of my time than waiting for everyone to agree they are useful.

That expectation does not change what I owe someone who uses Ze. I remain responsible for an outage, route leak or security hole in software I release, regardless of how the code was written. I expected Ze to be finished by now, but the issues we keep finding have taken longer to fix than I expected. The repository is open, with the tests and design notes available for scrutiny, and I would rather delay Ze than release it with problems I already know about.

*Last updated: 21 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/ai-slop-is-the-wrong-test.md).*
