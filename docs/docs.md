# Documentation

Everything written about Ze, organised by what you are trying to do. If you
are new, start at the top and work down. If you already know Ze, jump
straight to the reference.

## Start here

Learning-oriented pages that take you from nothing to a working setup.

- [Quickstart](guide/quickstart/): two BGP peers talking in under five minutes.
- [Install Ze](guide/ze-install/): the daemon on an existing Linux box, or a bootable appliance.
- [Configuration](features/configuration/): the one YANG model that describes everything Ze does.
- [CLI commands](features/cli-commands/): the SSH CLI, with diff, commit, and history.

## How-to guides

Task-oriented pages for when you have a specific job to get done and already
know your way around.

- [Migrating from ExaBGP](guide/exabgp-migration/): convert an ExaBGP config and run existing process scripts.
- [Firewall](guide/firewall/) and [policy routing](guide/policy-routing/).
- [OSPF](guide/ospf/), [IS-IS](guide/isis/), and [static routes](guide/static-routes/).
- [PPPoE](guide/pppoe/) and [L2TP](guide/l2tp/) access concentration.
- [Flow export](guide/flow-export/), [monitoring](guide/monitoring/), and [MRT analysis](guide/mrt-analysis/).
- [AS112](guide/as112/), [TACACS+](guide/tacacs/), and the [audit trail](guide/audit/).
- [VPP dataplane](guide/vpp/), [benchmarking](guide/benchmarking/), and [production diagnostics](guide/production-diagnostics/).

The [command reference](guide/command-reference/) lists every CLI command in one place.

## Reference

Information-oriented pages you look things up in. Accurate, complete, and
generated from the running system where possible, not written by hand.

- [Features](../features/): every capability, colour-coded by category and marked by status.
- [CLI reference](../cli/): all 350 commands, generated from the live binary.
- [Configuration reference](../config-reference/): the whole config as one searchable tree, from live YANG.
- [Dependencies](../dependencies/): every direct Go package and why it is there.

## Understand the design

Explanation-oriented pages for when you want to know why Ze is built the way
it is, not just how to use it.

- [Architecture](architecture/): how the engine, config model, plugins, and operator surfaces fit together.
- [Compare](../compare/): how Ze stacks up against BIRD, FRR, GoBGP, and others, including where it is still behind.
- [Performance](../performance/): measured BGP benchmarks, with the method, not marketing numbers.
- [Labs](../labs/): interop proof you can run yourself against real FRR, BIRD, strongSwan, and more.
- [Usage examples](../usage/): complete network deployment shapes, starting with Ze as an AS112 anycast DNS node.

## Keep up and get involved

- [Roadmap](../roadmap/): the path to the first release.
- [Changes](../changes/): what shipped, week by week.
- [Blog](../blog/): the longer story behind each week's work.
- [FAQ](../faq/): the questions people ask before they commit time to Ze.
- [Contribute](../contribute/): how the project takes code, bug reports, and interop results.
