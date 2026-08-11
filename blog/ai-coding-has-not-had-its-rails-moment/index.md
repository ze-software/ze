# AI coding has not had its Rails moment

*2026-08-10 by Thomas Mangin*

An agent can locate and open any file in a large repository in under a second. Nothing it finds there tells it how to work with what it has just opened: which design document explains what that code implements, which rule it is about to break, or which test would catch it if it gets that wrong. That knowledge exists. It lives in a wiki nobody updates, in a handful of pull request comments, and in the maintainer's head.

This is a problem about repository structure, and it is a good deal older than AI.

At the start of the 2000s, project layouts were not standard. At Exa, our initial convention put each project's configuration in `etc/`, its working data in `data/`, the reusable code in `lib/` and the rest in `src/`. It was not a one to one match with the Filesystem Hierarchy Standard (FHS), but changing `$ETC` and `$DATA` was enough to point a project at the right place on an installed system. During development, the repository acted as the installed root, leaving only a code-test loop.

The same thinking is visible in the [first ExaBGP commit from September 2009](https://github.com/Exa-Networks/exabgp/commit/5490f7baf5981279e2360d88c735570bc9f72532). It had `daemon`, `etc`, `lib` and `test` directories, and a test which announced a route to a Cisco 7204 and kept the connection alive.

Most repositories on GitHub are well organised today, for human developers and for the tools we already have. The reasoning behind that organisation is never written into the tree. People carried it instead. An agent arrives with none of that and no way to ask for it, and then we complain that it fights the developers instead of joining the team.

Rails faced the same problem twenty years ago, and it had to teach a whole community to follow the structure the framework expected. That is where I want to start.

*This article was co-authored with Claude. The argument and the conclusions are mine. Claude helped organise the material and draft the text.*

## Rails made the tree an interface

By 2009 this kind of layout was far more familiar, largely because Ruby on Rails had made project structure part of the framework and turned it into something people argued about. Its [official philosophy](https://guides.rubyonrails.org/getting_started.html#rails-philosophy) calls the idea Convention over Configuration, and running `rails new` creates recognised places for application code, configuration, database changes, libraries and tests.

Rails was probably not the first project to lay a tree out like that. It was the first to make the idea popular, and twenty years later the same thing is starting to happen around the plain Markdown files an agent reads. I come back to that at the end.

What Rails did was make its conventions consistent, generate them automatically and teach them to a large community. A developer entering an unfamiliar Rails application already knew where to look, and so did the Rails tools. The directory tree had become an interface shared by people and programs, and nothing built on top of it ever had to rediscover an application's shape.

The idea spread far beyond Rails. Plenty of ecosystems now ship a command which lays out a new project for you, such as `django-admin startproject`, `cargo new` or `ng new`. Generating a project tree is ordinary by now, and whatever an agent needs in that tree could be generated the same way.

What those commands generate is aimed at the compiler, the package manager and the human reader. Not one of them puts anything in the tree for an agent.

An agent needs the same interface a new developer needs, with one extra difficulty: it starts every session knowing nothing at all.

## The tooling improved and the repositories did not

AI coding has concentrated on the tool side. Harnesses keep getting better at tool use, planning, context management and coordination between agents, and the models improve every few weeks. The tooling looks the part now, with its animated terminals and neat progress trees, but the machinery underneath is still immature.

Stencil's [The harness problem](https://stencil.so/blog/the-harness-problem) measured something far more basic than planning or context management: whether a model can apply the edit it has already decided to make. Grok 4 failed half of its patches on their benchmark, EDIT-Bench has a single model above sixty per cent on realistic editing tasks, and changing nothing except the format of the edit moved sixteen models by fifteen points on average. Stencil sells that format, so their own figures deserve the usual caution, and an effect of that size is hard to dismiss.

A model which cannot reliably replace a line of text is not being held back by its understanding of the code. A model's ability is fixed the day it ships, and how much of that ability arrives in the repository depends on what we build around it. The edit format is the cheapest thing on that list, and a fair measure of how much better AI coding can get without waiting for new models.

A harness which edited perfectly would still walk into an arbitrary repository with no idea where anything is meant to live. It has to support every language and every kind of repository, so it stays general. It can search files, edit text and run commands. It cannot know why one project requires registration while another prefers a switch statement, and it cannot find an architectural decision the project never linked to its code. A rule which exists only in the maintainer's memory is beyond it entirely.

We keep improving the worker while leaving the workplace unexplained.

Frontier labs are well placed to fix half of this, because they build the model and the harness together and can train one for the other. The other half is not theirs to fix. Nobody at a lab can decide where your architectural decisions live, which patterns your project considers correct, or what counts as evidence that a change works. That has to happen inside the repository, and for now it has to be built by hand.

This is one reason opinions about AI coding are so divided. Some people see the failures of current harnesses and conclude that AI cannot produce serious software, while the people getting good results from it accept the awkwardness of today's tools as a permanent cost. Both positions treat the current state of the art as its final form.

Version control went through this. For most developers at the start of the 2000s, version control meant CVS, and CVS could not rename a folder, a serious limitation when the structure of a project was the thing you were working on. Enough engineers fought it to conclude that version control was ceremony which got in the way of writing software, and they said so for years. Subversion was the modern choice for anyone CVS had not already put off the idea entirely. It took Git, and the community which formed around it, before many of them looked again at something they had rejected on the evidence of one bad tool.

The distrust of AI-generated code has the same shape. Code arrives faster than anyone can read it, nothing in the repository tells the agent which rules it is expected to respect, and nothing comes back with the change to show that it did. That is a description of the tooling, and it is being taken as a verdict on the technology.

AI-assisted development reminds me of using the Internet at home over 64K between 1994 and 1996. I had already used a faster university connection, so home access was painful. It required technical skill and motivation, and bulletin board systems were still more popular than the Internet. Yet I loved it because the promise was obvious. ADSL and services such as BitTorrent and YouTube later made the Internet useful to far more people. Judging the Internet by 64K would have mistaken a temporary stage for the limits of the technology.

AI coding is at a comparable stage. It is already a large improvement for technical and motivated users who are willing to work around its limitations, and its ADSL moment has not arrived. Many of the AI development best practices which will make it ordinary are still to be invented.

Most of that missing work is not intellectually difficult. Stencil's fifteen point improvement took no model training and around three hundred dollars of benchmarking, so it was there for anyone willing to run the experiment. An edit format that every harness agrees on still does not exist.

In 1999 and 2000, working for an ISP felt like having a superpower, perhaps the way working for a frontier AI lab feels now, with unlimited tokens to spend on whatever you are trying to build.

## Other engineers are finding the same shape

This problem is wider than one project. Ze, the network operating system I have been building, is where I met it, and much larger engineering organisations are meeting it too.

Cloudflare published [How Cloudflare enforces engineering standards using AI](https://blog.cloudflare.com/engineering-standards-enforcement/) in August 2026. Before building what it calls the Cloudflare Codex, its engineering guidance lived in formal documentation, repository files, chat threads and the accumulated knowledge of individual engineers. Developers spent time searching and could not always tell whether an answer was current, authoritative or relevant. We found the same thing while building Ze.

Cloudflare turned its standards into structured RFCs. Requirements use `MUST` and `SHOULD`. Each statement has a stable name which survives edits to the document around it. Agents receive a compact set of relevant statements first and load the complete RFC only when they need more. AI reviewers check code and technical designs, and rules which ordinary software can recognise are moving into fast linters.

Ze arrived at a remarkably similar shape inside a single repository: instructions with stable path names, a compact index which routes a task to the rule that governs it, source files pointing at their design documents, edit hooks which reject common mistakes while the file is being written, and larger checks before a change is accepted.

We reached the same conclusion, and most of the writing had been done for us. Ze implements RFCs, and an RFC already says what the code has to do, section by section, in the same `MUST` and `SHOULD` terms ([RFC 2119](https://www.rfc-editor.org/rfc/rfc2119.html)). We put our own rules into that form as well. The work was in the checking. For every `MUST`, Ze wants one test showing that valid data is handled correctly and one showing that invalid data is rejected, and a requirement with no test is listed as a gap. [The proof is the expensive part](../the-proof-is-the-expensive-part/) explains how that list is built and kept honest.

[The repository is half the AI harness](../the-repository-is-the-ai-harness/) goes through all of it, including what it costs to keep running. Cloudflare is applying this across a large engineering organisation and Ze is doing it inside an open-source project. Two efforts that different, converging on the same answer, is useful evidence.

## The convention will come from a project

AI-assisted development is beginning to discover its Convention over Configuration, and we do not yet know which project will make it viral. Its choices will have to be good enough, without having to be the best available. What decides the race is reaching a large audience first, and I expect the winner to be a command which sets a repository up for an agent, the way `rails new` sets one up for a developer. Many people meet the same pain, the available tools make an answer possible, several engineers independently build something similar, and one implementation gets the memorable name and becomes the example everybody remembers.

Andrej Karpathy gave a small demonstration of that with his [LLM Wiki gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f). It describes an LLM maintaining a wiki made from ordinary Markdown files, with Obsidian as the editor. Raw sources stay separate, the agent maintains cross-linked pages, an `index.md` helps it find the right information, and an `AGENTS.md` or `CLAUDE.md` explains the structure. More complicated search can wait until the collection needs it.

The idea became extremely popular the moment he shared it, and many people, including yours truly, had already reached the same answer independently. Ze was already using Markdown for its rules, designs and working memory, maintaining cross-links and generating indexes, all of it in the same text editors we write the code in. What Karpathy added was a clear explanation and a large audience, and for many people his version will become the version they know and copy.

A reference version gets copied whole. Excellent choices spread, awkward ones spread with them, and familiarity turns the whole set into the convention.

ExaBGP could run straight out of its project folder because the design let the program find its environment, with no installation step in between. An AI-ready repository has to do the same thing for an agent, and let it find the project's architecture without already understanding it. That environment includes the source code, its relationships, the designs, the tests, the checks and the failure messages. Building all of that by hand, for one repository, is what I have spent this year doing.

The machinery which supports Ze's development is built, and I have just finished updating it to work well with Opus 5. Ze is a network operating system, and I am going back to writing the router rather than the engine which produces it.
