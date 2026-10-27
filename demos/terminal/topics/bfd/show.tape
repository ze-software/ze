Hide
Type "ze-demo run bfd-failover await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze-demo card topic-bfd proof"
Enter
Sleep 6s
Type "ze cli -c 'show bfd profile name fast | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show bfd session address 172.29.0.2 | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show bgp peer 172.29.0.2 detail | no-more'"
Enter
Sleep 4s
Type "ip netns exec showcase-frr vtysh -c 'show bgp neighbor 172.29.0.1 json'"
Enter
Sleep 4s
Type "ze-demo card topic-bfd failure"
Enter
Sleep 6s
Type "ze-demo run showcase-lab control frr down"
Enter
Hide
Type "ze-demo run bfd-failover await failed"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show bfd session address 172.29.0.2 | no-more'"
Enter
Sleep 3s
Type "ze cli -c 'show bgp peer 172.29.0.2 detail | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show bgp peer 172.29.0.2 history | no-more'"
Enter
Sleep 4s
Type "ze-demo card topic-bfd fix"
Enter
Sleep 6s
Hide
Type "ze-demo run bfd-failover await retired"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show bfd sessions | no-more'"
Enter
Sleep 4s
Type "ze-demo run showcase-lab control frr up"
Enter
Hide
Type "ze-demo run bfd-failover await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show bfd session address 172.29.0.2 | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show bgp peer 172.29.0.2 detail | no-more'"
Enter
Sleep 4s
Type "cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid"
Enter
Sleep 2s
Type "ze-demo card topic-bfd recap"
Enter
Sleep 6s
