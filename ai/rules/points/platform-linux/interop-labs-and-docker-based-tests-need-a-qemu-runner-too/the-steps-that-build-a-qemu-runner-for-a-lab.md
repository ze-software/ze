---
kind: directive
level: MUST
stage:
---
A lab that needs a QEMU runner MUST be built with all four steps below. Three
of them fail closed on their own; the fourth, registration, does not, so a lab
that skips it is invisible rather than red.

1. **Native netns evidence:** implement the lab under `internal/le` and register a named `./le deployment <verb>` or `./le qemu <verb>` action. Run Ze and the peer daemon in separate network namespaces joined by a veth, without Docker.
2. **Peer from Alpine packages:** install the peer daemon through the `packages` parameter of `./le qemu run`, or declare it in the dedicated native QEMU action. Use the same packaged peer in the Docker and QEMU proofs where Alpine supplies it.
3. **Runtime kernel, always:** pass Ze's staged runtime kernel through `./le qemu run kernel <vmlinuz>`. `Run.assertRuntimeKernel` refuses a guest whose `uname -r` does not match `internal/appliance/kernel.version`. Add every required `CONFIG_*` symbol to `gokrazy/kernel/runtime.config` and `gokrazy/kernel/runtime.require`.
4. **Registered action:** add the feature action to the owning Go action table and expose it through `./le qemu` or `./le deployment`. The bare area command is the inventory, and it MUST list the new action.
