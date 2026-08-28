---
kind: table
level:
stage:
---
| Proof | What it does | Use it for |
|-------|--------------|------------|
| `./le qemu vpp-hugepages-test` | builds a real image via `ze appliance build`, boots it in QEMU, asserts the kernel cmdline and the reserved hugepage count | the default boot proof |
| `./le deployment gokrazy-l2tp-ppp-test` | builds the appliance and boots it against a real LAC | the L2TP path |
| ~~`test/appliance/serial-login.ci`~~ | **boots nothing.** Its header says the QEMU plan applies "when appliance serial test infrastructure is ready"; it asserts the argv[0] shell-invocation gate offline | never cite it as a boot proof |
