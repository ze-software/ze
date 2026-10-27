Hide
Type "ze-demo run ospf-adjacency await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze-demo card topic-ospf proof"
Enter
Sleep 6s
Type "ze cli -c 'show ospf neighbor detail | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show ospf interface detail | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show ospf database router | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show ospf route | no-more'"
Enter
Sleep 4s
Type "ze-demo card topic-ospf failure"
Enter
Sleep 6s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-ospf mismatch
Type "load file absolute merge /src/demos/terminal/topics/ospf/3-mismatch.conf"
Enter
Wait+Screen /merged from \S*3-mismatch\.conf/
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
Hide
Type "ze-demo run ospf-adjacency await mismatch"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show ospf neighbor detail | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show ospf interface detail | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show metrics name ze_ospf_packets_dropped_total label interface frr0 label reason hello-interval | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show ospf route | no-more'"
Enter
Sleep 4s
Type "ze-demo card topic-ospf fix"
Enter
Sleep 6s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-ospf recovery
Type "load file absolute merge /src/demos/terminal/topics/ospf/4-recovery.conf"
Enter
Wait+Screen /merged from \S*4-recovery\.conf/
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
Hide
Type "ze-demo run ospf-adjacency await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show ospf neighbor detail | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show ospf database router | no-more'"
Enter
Sleep 4s
Type "ze cli -c 'show ospf route | no-more'"
Enter
Sleep 4s
Type "cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid"
Enter
Sleep 2s
Type "ze-demo card topic-ospf recap"
Enter
Sleep 6s
