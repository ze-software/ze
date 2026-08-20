---
kind: table
level:
stage:
---
| Target | What it runs | When required |
|--------|-------------|---------------|
| `make ze-netns-test` | `firewall` `policy` `ospf` `ospfv3` suites under `ZE_TEST_NETNS=1` | Any change to nft/FIB/OSPF kernel programming |
| `make ze-netns-plugin-test` | `system-kernel-log-show`, which needs CAP_SYSLOG to read `/dev/kmsg` | Any change to `readKmsg` |
