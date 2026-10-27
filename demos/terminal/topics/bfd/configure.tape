# Shared by the standalone topic and the continuous showcase.
Type "ze-demo card topic-bfd overview"
Enter
Sleep 6s
Hide
Type "ze-demo run bfd-failover await absent"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid"
Enter
Sleep 2s
Type "ze cli -c 'show bfd sessions | no-more'"
Enter
Sleep 4s
Type "ip netns exec showcase-frr vtysh -c 'show bgp neighbor 172.29.0.1 json'"
Enter
Sleep 4s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-bfd profile
Type "load file absolute merge /src/demos/terminal/topics/bfd/1-profile.conf"
Enter
Wait+Screen /merged from \S*1-profile\.conf/
Sleep 3s
Card topic-bfd peer
Type "load file absolute merge /src/demos/terminal/topics/bfd/2-peer.conf"
Enter
Wait+Screen /merged from \S*2-peer\.conf/
Sleep 3s
Type "show | compare"
Enter
Sleep 6s
Type "commit now"
Enter
Wait+Screen /Session committed/
Sleep 3s
Type "exit"
Enter
Wait+Screen /operational\]/
Type "exit"
Enter
Wait+Screen /\$ /
