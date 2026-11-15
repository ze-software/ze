# Frequently asked questions

These answers cover what Ze does, its release status, and what adopting it involves. If your question is missing, you can ask on [Discord](https://discord.gg/T8s7CjPDne) or open an [issue](https://github.com/ze-software/ze/issues).

## Product and readiness

Start here if you are deciding whether Ze belongs in your network.

**What is Ze?**

Ze is an open-source configuration and protocol engine, also available as a Linux network operating system. It speaks BGP, manages network interfaces, updates the forwarding table, and lets you manage the same configuration through SSH, a web interface, APIs, and AI tools using MCP (Model Context Protocol).

The core manages configuration, plugins, and communication between components, while individual subsystems implement the network protocols. Each subsystem defines its configuration in a YANG schema. Ze uses those definitions for its command syntax, completion, validation, web editor, reference pages, and MCP tools.

**Is Ze ready for production?**

No. Ze is pre-release software. The BGP engine has automated tests, but production exposure remains limited, and both APIs and configuration syntax can still change.

A lab is a useful place to try Ze today, whether you want to build a route server, migrate an ExaBGP configuration, or test it against FRR or BIRD. Reports of failures help us find what those tests miss. The [roadmap](https://ze-software.net/project/roadmap/) lists the release work, while the [quality pages](https://ze-software.net/quality/) describe the available tests and their limits.

**What can Ze actually do today?**

Ze implements BGP and manages Linux interfaces and forwarding tables. You can change configuration through SSH, a web interface, or APIs, review and commit those changes, and roll them back. It also provides monitoring and a plugin system.

OSPF, IS-IS, MPLS, firewall, VPN, PPPoE, and L2TP code also exists, but those areas remain experimental. Use the [feature catalog](https://ze-software.net/features/) for current status instead of treating this answer as a compatibility matrix.

**Why would I use Ze instead of BIRD, FRR, or GoBGP?**

A mature daemon is the better choice when you need an established production platform today. Ze may be worth evaluating if you want a shared configuration model across its CLI, web interface, and APIs, or if its plugins and ExaBGP migration tools fit your deployment. Its [quality pages](https://ze-software.net/quality/) also make the test methods and evidence available for review.

Each subsystem supplies a YANG model that defines its configuration and supplies the CLI, validation, web editor, reference pages, and MCP tools. The [comparison pages](https://ze-software.net/compare/) describe the differences between Ze and other projects, including features Ze still lacks.

## Running and adopting Ze

Practical questions about trying Ze before you commit a network to it.

**Where does Ze run?**

The daemon runs on Linux under systemd or another process supervisor. Development builds work on macOS and Linux with the supported Go toolchain. Windows is not a supported development or runtime platform.

Start with the [quickstart](https://ze-software.net/guides/quickstart/). Use the [appliance lab](https://ze-software.net/labs/appliance-install/) when you want an immutable boot image for a VM or dedicated hardware.

**I run ExaBGP. Can I move to Ze?**

Yes. Ze includes `ze exabgp migrate` for configuration conversion and a compatibility bridge that lets existing ExaBGP process scripts run while you port them.

The [ExaBGP migration guide](https://ze-software.net/use-cases/exabgp-migration/) covers conversion, known differences, and the point where a native Ze plugin becomes the better choice.

**The daemon or the appliance: which should I run?**

Start with the daemon. It fits into Linux infrastructure you already supervise and is easier to inspect while Ze is still pre-release.

The appliance packages the same binary and configuration into a read-only system with automatic supervision, no package manager, and no interactive shell. Use it when you want a purpose-built router image rather than another managed Linux host.

**Will my configuration keep working as Ze changes?**

Configuration syntax can change before the first release, and changes are described in the [change log](https://ze-software.net/project/changes/). An existing configuration may need editing when you update Ze. Silent reinterpretation is a bug.

Configuration stability is an explicit milestone on the [roadmap](https://ze-software.net/project/roadmap/).

**Does Ze need an LLM to run?**

No. Ze runs without an AI service. MCP is an optional operator interface generated from the same schemas and command catalog as the CLI and web interface. It lets an authorized assistant discover and operate the capabilities compiled into a running daemon.

## Project and support

How the project is licensed, maintained, and supported.

**What does the AGPLv3 license mean for me?**

You can run, inspect, modify, and redistribute Ze under the [GNU Affero General Public License v3](https://ze-software.net/license/). Running an unmodified Ze for your own network does not require publishing your configuration.

If you modify Ze and let users interact with that modified version over a network, the AGPL requires you to offer those users the corresponding source. Read the license itself when the distinction matters to a deployment.

**What does the contributor agreement grant?**

A signed-off commit (`git commit -s`) confirms agreement to the [Contributor License Agreement](https://ze-software.net/contribute/). You keep your copyright.

The agreement lets the maintainer offer Ze under additional license terms, including a commercial license. Every contribution remains available to the public under AGPLv3. The [contributor guide](https://ze-software.net/contribute/guide/) covers the submission process.

**Who builds Ze, and how is it funded?**

Thomas Mangin develops Ze, with engineering time supported by [Exa Networks](https://exa.net.uk). The ISP has backed the work since it began with ExaBGP in 2009.

Ze currently has no subscription, paid support tier, or separate commercial entity. The [contribute page](https://ze-software.net/contribute/) explains how the project is maintained and how to help.

**How is AI used to develop Ze?**

AI coding agents write Ze's code under a process we developed to guide their work and check the result. We agree on a specification before implementation, write tests first, and ask a separate agent to review the change. Automated checks enforce the repository's rules before a change is committed. Keeping the instructions beside the code gives each new session the same requirements. [How Ze is developed with AI](https://ze-software.net/project/ai-development/) explains how this process developed.

As with human development, the first version is often not fully correct, and we improve it through repeated testing and review. Checking a feature thoroughly takes time and computing resources. For an RFC, every requirement has to be identified and tested, and each test has to detect a deliberate fault in the behavior it checks. That coverage keeps growing after the first implementation is published. The [RFC compliance pages](https://ze-software.net/quality/rfc-compliance/) show what the tests establish and what is still missing.

The website is harder to check automatically. An RFC can define behavior precisely enough for a test, while free text leaves more room for unclear explanations and mistakes. Most of the site's figures, feature lists, and references are generated from the code, but the prose still needs review and is likely to contain more mistakes than the code. We revise it as we find them. If you spot a mistake on the site or in Ze, we would appreciate a report on [Discord](https://discord.gg/T8s7CjPDne) or the [issue tracker](https://github.com/ze-software/ze/issues).

**How do I get help or report a security problem?**

Use [Discord](https://discord.gg/T8s7CjPDne) for discussion and the [issue tracker](https://github.com/ze-software/ze/issues) for reproducible bugs and feature requests.

Follow the [security policy](https://ze-software.net/security/) for anything security-sensitive. Report vulnerabilities privately instead of opening a public issue.
