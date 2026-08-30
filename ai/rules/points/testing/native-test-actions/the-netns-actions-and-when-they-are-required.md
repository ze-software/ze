---
kind: directive
level: MUST
stage:
---
**A change matching a "when required" cell MUST run that action.** How the netns
actions obtain and drop their privilege is `docs/functional-tests.md`, "Netns
launch mode".

| Action | What it runs | When required |
|--------|--------------|---------------|
| `./le qemu netns-test suites firewall,policy,ospf,ospfv3` | Kernel-programming functional suites | Changes to nft, FIB, or OSPF kernel programming |
| `./le qemu run command '<focused Go test>'` | A focused capability-dependent package test | Changes to kernel log or other guest-only behavior |
