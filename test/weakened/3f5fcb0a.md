| Test | Reason |
|---|---|
| withVethPeerNamespace | Not weakened: its body moved into withVethPair(cidrA, cidrB) unchanged and withVethPeerNamespace now calls it with the same 192.0.2.1/24 and 192.0.2.2/24 addresses, so every caller builds the identical lab. The only behavioural change is fresh veth names per lab, which stops a re-created veth resolving to a deleted ifindex. |
