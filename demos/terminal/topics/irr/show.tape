Type "ze-demo card topic-irr proof"
Enter
Wait+Screen /====/
Sleep 5s
Type "ze cli -c 'show bgp irr | yaml | no-more'"
Enter
Wait+Screen /status: ok/
Type "ze cli -c 'show bgp irr prefix customer | yaml | no-more'"
Enter
Wait+Screen /2001:db8::\/32/
Sleep 3s
Type "ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'"
Enter
Wait+Screen /accepted: true/
Type "ze cli -c 'show bgp irr check customer 192.168.0.0/24 | yaml | no-more'"
Enter
Wait+Screen /accepted: false/
Type "ze cli -c 'show bgp adj-rib-in | no-more'"
Enter
Wait+Screen /10\.0\.0\.0\/24/
Wait+Screen /10\.0\.1\.0\/24/
Sleep 4s
Type "ze-demo card topic-irr failure"
Enter
Wait+Screen /====/
Sleep 5s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-irr wrong
Type "load file absolute merge /src/demos/terminal/topics/irr/4-wrong-as-set.conf"
Enter
Wait+Screen /merged from \S*4-wrong-as-set\.conf/
Type "show | compare"
Enter
Wait+Screen /AS-V4ONLY/
Type "commit now"
Enter
Wait+Screen /Session committed/
Type "exit"
Enter
Wait+Screen /operational\]/
Type "exit"
Enter
Wait+Screen /\$ /
Hide
Type "ze-demo run showcase-lab control customer reannounce"
Enter
Wait+Screen /Customer routes announced/
Type "ze-demo run irr-filter await wrong"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'"
Enter
Wait+Screen /accepted: false/
Type "ze cli -c 'show bgp irr prefix customer | yaml | no-more'"
Enter
Wait+Screen /192\.0\.2\.0\/24/
Type "ze cli -c 'show bgp adj-rib-in | no-more'"
Enter
Wait+Screen /192\.0\.2\.0\/24/
Sleep 4s
Type "ze-demo card topic-irr recovery"
Enter
Wait+Screen /====/
Sleep 5s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Card topic-irr as-set
Type "load file absolute merge /src/demos/terminal/topics/irr/2-as-set.conf"
Enter
Wait+Screen /merged from \S*2-as-set\.conf/
Type "show | compare"
Enter
Wait+Screen /AS-TEST/
Type "commit now"
Enter
Wait+Screen /Session committed/
Type "exit"
Enter
Wait+Screen /operational\]/
Type "exit"
Enter
Wait+Screen /\$ /
Type "ze cli -c 'update bgp irr as-set AS-TEST | yaml | no-more'"
Enter
Wait+Screen /refreshed:/
Type "ze-demo run showcase-lab control customer reannounce"
Enter
Wait+Screen /Customer routes announced/
Hide
Type "ze-demo run irr-filter await healthy"
Enter
Wait+Screen /Topic ready/
Type "clear"
Enter
Show
Type "ze cli -c 'show bgp irr check customer 10.0.0.0/24 | yaml | no-more'"
Enter
Wait+Screen /accepted: true/
Type "ze cli -c 'show bgp adj-rib-in | no-more'"
Enter
Wait+Screen /10\.0\.0\.0\/24/
Wait+Screen /10\.0\.1\.0\/24/
Sleep 5s
