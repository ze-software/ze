---
title: AI coding has not had its Rails moment
date: 2026-08-10
author: Thomas Mangin
description: I set out to build a router and have spent much of the year building its development environment. Rails made shared conventions ordinary, and I think AI coding is due something similar.

deck: An agent can find my code more easily than it can find the reasons behind it. I have been teaching one repository to explain itself, and I do not think every project should have to invent this again.

image: assets/blog/ai-coding-has-not-had-its-rails-moment.svg
image-dark: assets/blog/ai-coding-has-not-had-its-rails-moment-dark.svg
image-alt: A recognised generated repository tree is compared with an arbitrary tree whose design, rules and tests sit apart; a shared convention joins project meaning to predictable locations.

---

I set out to build Ze, a network operating system, and have spent much of this year building the machinery needed to develop it with AI. There is a certain irony in using a tool to save time and then spending so much time explaining the project to it. An agent can find the code readily enough, but the reasons it is written that way are another matter.

Some of those reasons were in documents and some were still in my head. I have had to write them down and arrange them so that an agent can find the ones relevant to its change, along with the checks that change must pass. A better model is welcome, but it cannot read a decision I never recorded.

I do not think we should all have to build this arrangement independently. We already expect a framework to generate a project whose layout other developers recognise, and Rails showed how useful that familiarity could become. AI coding has room for something similar, and I am rather looking forward to it.

*This article was co-authored with Claude and revised with OpenAI Codex. The argument and the conclusions are mine. Claude helped organise the material and draft the original text.*

## We already knew where things belonged

At Exa in the early 2000s, we had a layout we carried between projects. Configuration went in `etc/`, working data in `data/`, reusable code in `lib/` and the rest in `src/`. It did not map exactly to the Filesystem Hierarchy Standard, but changing `$ETC` and `$DATA` pointed the project at the right places on an installed system.

During development the repository itself acted as the installed root. I could change the code and try it without installing it first, and the configuration needed to run it was in the same tree. This was a fairly modest convention, but it removed an unnecessary interruption every time I went round the code-test loop.

The [first ExaBGP commit, from September 2009](https://github.com/Exa-Networks/exabgp/commit/5490f7baf5981279e2360d88c735570bc9f72532), contains `daemon`, `etc`, `lib` and `test` directories. According to its commit message, the `supervisor.py` test could announce a route to a Cisco 7204 and keep the connection alive. There was already enough there to try it, and we knew where to look for what it needed.

With Rails, that familiarity extended to a community. Its [Convention over Configuration philosophy](https://guides.rubyonrails.org/getting_started.html#rails-philosophy) means that `rails new` gives application code and configuration recognised homes, along with database migrations, libraries and tests. The framework uses that arrangement too, so people and tools share the same expectations. Someone entering an unfamiliar Rails application already knows quite a lot before reading its first model.

Younger programmers who have never used Rails will recognise the benefit in React projects. React leaves more of the surrounding structure to its ecosystem, but familiar component conventions still help when entering a codebase for the first time. We take this for granted now, which is a good outcome for a development tool.

For an agent, I would like that familiarity to extend to the project's reasoning. Finding the source file is useful, but so is finding why it has to behave a particular way and what would demonstrate that a change broke that behaviour. The maintainer still has to supply the answers. A shared arrangement would at least save every agent, and every developer joining the project, from first having to discover where the answers were put.

## Some of the problems are surprisingly basic

Much of the attention in AI coding goes to the model and its harness, the program which supplies tools and manages the session. Planning has improved, and agents can coordinate with other agents, yet applying an edit can still fail before any of that cleverness gets much use.

Stencil's [The harness problem](https://stencil.so/blog/the-harness-problem) compared edit formats across sixteen models, using 180 tasks per run and three runs. Its tasks introduced mechanical bugs into React source files, with success judged against the original file. Grok 4 had a 50.7 per cent patch failure rate in that evaluation, and changing the edit format alone took GPT-5.1 Codex Mini's pass rate from 60.0 to 77.5 per cent.

Stencil sells the format, and undoing synthetic mutations is a narrower exercise than maintaining a project, so I would keep both in mind when reading those figures. Even so, the same model completed more of the tasks after a change to its editing interface. Stencil reports spending about $300 on the benchmarking. That is an experiment people outside a frontier lab can afford, with no need to train a new model.

I find this encouraging. Training the model is expensive and outside my control, while changing its tools is something we can do. If an agent wastes its time fighting a patch format, I would rather fix the format than accept the failure as a permanent limitation of AI coding. There is useful progress available in quite ordinary engineering.

A perfect editing tool would still leave the agent wondering why Ze uses registration where another project uses a central switch statement. A general-purpose harness has to work with different architectures and languages, and cannot choose Ze's design for me. Search will find the implementation, but the explanation has to exist somewhere for it to be found.

`AGENTS.md` and `CLAUDE.md` give us an agreed place to start. Behind that file, each maintainer is still organising the rest by hand. It is easy enough to write a long introduction to the project; giving the agent a short route to the rule which governs the particular change is more useful, and takes more thought.

The labs can train their models to use their harnesses better. They cannot supply my architectural decisions or decide what I should accept as proof of a routing change. That information belongs in the repository, and there should be a way to organise it which people recognise across projects. I have been putting it together for Ze because I need it, but repeating the exercise separately in every repository seems a poor use of everyone's time.

## I remember using the awkward versions

CVS was a familiar frustration at the start of the 2000s. Even renaming a directory was awkward: its [manual describes moving the files individually](https://www.gnu.org/software/trans-coord/manual/cvs/html_node/Moving-directories.html), or changing the repository directly and accepting the consequences for other working copies and old releases. When reorganising a project was what you were trying to do, the version control system had become part of the problem.

I remember resistance to version control as ceremony which got in the way of writing software. Subversion gave us a more modern choice, and later Git and the community around it gave people more reasons to reconsider. Someone could have a justified complaint about CVS and carry it for years after the reason for it had gone away.

I see something similar in the arguments about AI coding. A maintainer receives a large patch and has to discover which project rules it respects and whether any of its tests prove anything. It is perfectly reasonable to be annoyed. It seems less reasonable to assume that this is the best way AI development will ever work, or that those of us getting useful results must put up with all the awkwardness indefinitely.

Using AI now also reminds me of having Internet access at home over 64K between 1994 and 1996. I had already used a faster university connection, so I knew what I was missing. Getting useful things done at home required technical skill and motivation, and I loved it anyway. There was enough there to see why it was worth the inconvenience.

ADSL made that connection practical for more people, and later BitTorrent and YouTube gave them more reasons to want it. The limitations of my home connection were frustrating precisely because I knew there was more I could do. I would have missed a great deal by deciding to wait until it was all easy.

In 1999 and 2000, working for an ISP felt like having a superpower. I imagine working at a frontier AI lab now, with tokens available for whatever you want to try, might feel rather similar. This is speculation, but I recognise the appeal of being able to experiment freely instead of fitting each attempt into what limited access permits.

So I am quite happy to use AI while the surrounding tools are still awkward. I expect them to improve, and shared repository conventions seem an obvious place to help that happen. We have the files and the links already; the interesting part is agreeing enough about their use that the next project can start further along.

## There are already examples to copy

Cloudflare's August 2026 article, [How Cloudflare enforces engineering standards using AI](https://blog.cloudflare.com/engineering-standards-enforcement/), described a problem I recognised. Guidance was spread between formal documentation and repository files, with more in chat threads and engineers' heads. Finding an answer did not necessarily tell an engineer whether it was current or whether it was the answer they were supposed to follow.

Cloudflare organised its standards as RFCs, with `MUST` and `SHOULD` carrying their [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119.html) meanings. Statements have stable identifiers, and agents can load a compact set before asking for the full documents. They can use those statements to review code and designs, while requirements ordinary software can check are also being moved into linters. Approval of a standard and the decision to make it blocking remain separate steps.

Ze has arrived at a related arrangement within one repository. Its task-oriented `ai/INDEX.md` points to rules and designs, and source annotations link code back to the documents which explain it. Someone learning the modular core is directed to `ai/patterns/registration.md`; implementing an RFC has a different route. [The repository is half the AI harness](../the-repository-is-the-ai-harness/) describes this and the feedback agents receive during development.

Protocol RFCs were a useful starting point for Ze because many of the requirements were already explicit. We ask for tests of valid and invalid behaviour against them, then have to check that each test demonstrates the rule it is attached to. Putting a requirement beside a passing test is easy enough, and can be quite misleading if the test would also pass a broken implementation. I wrote about that in [The proof is the expensive part](../the-proof-is-the-expensive-part/).

Cloudflare is dealing with engineering standards across a large organisation, while I am organising one open-source project. I find it encouraging that both have led towards explicit requirements which agents can find and people can check. There is enough in these examples for others to compare the choices and reuse what suits them, without starting with an empty `AGENTS.md`.

Andrej Karpathy's [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) gave me much the same feeling. It describes a knowledge base with raw sources kept separately from cross-linked Markdown pages, an `index.md` for discovery and an `AGENTS.md` or `CLAUDE.md` explaining the conventions. I recognised much of what we had been doing in Ze, using ordinary files and links.

Having a recognisable example helps more than it may first appear. Several people can independently arrive at similar arrangements and still have trouble explaining them to everyone else. A project people can point to gives those choices a name and a form they can copy, including the parts they might otherwise have done slightly differently. Once people know where things belong, those small differences may no longer be worth maintaining.

I am inclined to think AI-assisted development will get its Convention over Configuration this way, through a project people choose to copy. Perhaps it will come with a command which sets up a repository for an agent, much as `rails new` sets up an application. The useful example will have to carry an actual project's reasoning, so people can see how to supply their own rather than fill in a few empty headings.

It will still be my job to decide which dependencies Ze permits and explain why an implementation was rejected. I would gladly stop inventing where to put those decisions, and harness authors would gain a convention they could support across projects. ExaBGP could run from its project folder because the layout let it find its environment; I would like an agent to arrive in a repository with the same familiarity about how development is done there.

The machinery supporting Ze's development is now in place, and I have finished the update for Opus 5. Time to get back to writing the router. I had not intended to spend the year building the tools used to build it.

*Last updated: 11 September 2026. The history of this article is available in the [project's Git repository](https://github.com/ze-software/ze/commits/main/website/blog/posts/ai-coding-has-not-had-its-rails-moment.md).*
