# Project overview

Follow Ze from its ExaBGP roots to a network operating system: what works today, what is changing, and what remains before the first release.

## Where Ze stands

Ze brings routing protocols, Linux networking and operator tools together around one configuration. It grew from ExaBGP's programmable BGP model, with a broader job: configure the network, inspect its state, and expose the same operations to people and automation.

Ze is pre-release. You can explore it in a lab today; the available capabilities and their test evidence vary by feature.

 [Why Ze, and where it fits](https://ze-software.net/project/why-ze/)

Recent work

## Latest development

2026-11-09 – 2026-11-23 (15 days)

BGP accepts RPKI Invalid routes unless told otherwise and sets itself as next hop towards eBGP peers, gNMI…

 [Read the latest update](https://ze-software.net/project/changes/2026-11-09/)

[Browse all update reports](https://ze-software.net/project/changes/)

Pre-release

## Before the first release

The remaining work includes correctness fixes, validation of existing behavior, and release delivery. The roadmap separates these from later features and proposals.

Work-item counts describe the backlog. They do not measure effort or production readiness.

 [Release roadmap](https://ze-software.net/project/roadmap/) [Inspect the test evidence](https://ze-software.net/quality/)

A recorded session on a real lab router

## See Ze running

Watch an ExaBGP migration, inspect BGP peers, then configure, diagnose and repair services while the same router keeps running. The recordings include the commands and their output, with transcripts you can read at your own pace.

 [Watch Ze in action](https://ze-software.net/#product-terminal) [Try the BGP quickstart](https://ze-software.net/guides/quickstart/)

[Open the continuous configuration recording and transcript](https://ze-software.net/demos/terminal/#continuous-live-configuration-showcase)

 [![Ze's terminal dashboard showing BGP peers, session states and message counters](https://ze-software.net/assets/product-dashboard.webp) Preview from Ze's published terminal recording. Open the player to watch.](https://ze-software.net/#product-terminal)

Recorded achievements

## How we got here

These recent landmarks mark capabilities that arrived in Ze. The full timeline and project history explain how the system developed.

1. 2026-10-25

 ### LAN caching DNS resolver

`service dns-resolver` answers DNS queries from allowed LAN and subscriber prefixes over UDP and TCP, caches answers, and forwards cache misses to configured upstream servers. DNSSEC validation is still pending.
2. 2026-10-25

 ### Shared route policy

 Named filters move into a shared `policy` tree, with import chains at the Loc-RIB and export chains for IS-IS and OSPF redistribution.
3. 2026-10-05

 ### Ze appliance kernel

 Appliance images boot Ze's own Linux kernel, with the routing and VPN support the system needs, packaged with each image.

 [Explore the milestone timeline](https://ze-software.net/project/milestones/) [From ExaBGP to Ze](https://ze-software.net/project/history/)

## How Ze is built

AI coding agents write most of Ze's code within a framework of specifications agreed before implementation, tests written before code, independent reviews and commit checks. That process is iterative: code and the checks that exercise it improve as problems are found. Checking a first version against every requirement takes time and resources, so published code can still be imperfect.

Most of this site's information is generated from the code. Its free text has more room for mistakes than protocol behavior defined by an RFC, so reports of unclear or incorrect information help improve it.

 [How Ze is developed with AI](https://ze-software.net/project/ai-development/) [**Engineering blog**Longer accounts of the design and development.](https://ze-software.net/blog/) [**Talks**Presentations about Ze and ExaBGP.](https://ze-software.net/talks/) [**Development activity**Repository history and the scope of the code.](https://ze-software.net/project/activity/)

## Get involved

Try Ze in a lab and report a failure with its configuration, trace or transcript. Documentation corrections, interoperability results, tests and focused code changes all help the project move forward.

 [Contribute to Ze](https://ze-software.net/contribute/)
