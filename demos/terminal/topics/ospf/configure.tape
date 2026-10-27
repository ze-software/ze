# Shared by the standalone topic and the continuous showcase.
Type "ze-demo card topic-ospf overview"
Enter
Sleep 6s
Hide
Type "ze-demo run ospf-adjacency await absent"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "cat /src/tmp/terminal-demos/state/showcase-lab/ze.pid"
Enter
Sleep 2s
Type "ze cli -c 'show ospf neighbor | no-more'"
Enter
Sleep 4s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-ospf area
Type "load file absolute merge /src/demos/terminal/topics/ospf/1-area.conf"
Enter
Wait+Screen /merged from \S*1-area\.conf/
Sleep 3s
Card topic-ospf interface
Type "load file absolute merge /src/demos/terminal/topics/ospf/2-interface.conf"
Enter
Wait+Screen /merged from \S*2-interface\.conf/
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
