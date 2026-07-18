Output artifacts/rib-fib.webm
Source common.tape

Hide
Type "export ZE_CONFIG_DIR=/src/tmp/terminal-demos/state/rib-fib/config ZE_INIT_INPUT=/src/tmp/terminal-demos/state/rib-fib/init.input ZE_SSH_PASSWORD=secret123 SSHPASS=secret123"
Enter
Type "/src/demos/terminal/rib-fib/run.sh prepare"
Enter
Wait+Screen /RIB\/FIB demo prepared/
Type "ze init < $ZE_INIT_INPUT >/dev/null"
Enter
Type "/src/demos/terminal/rib-fib/run.sh start"
Enter
Wait+Screen /RIB\/FIB demo ready/
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh rib-fib intro"
Enter
Show
Sleep 11s

Hide
Type "clear"
Enter
Show
Type "ze cli -c 'request bgp rib inject 192.0.2.10 ipv4/unicast 198.51.100.0/24 origin igp nexthop 127.0.0.1 med 42'"
Enter
Sleep 4s
Type "ze cli -c 'show bgp rib best prefix 198.51.100.0/24 | no-more'"
Enter
Wait+Screen /198.51.100.0\/24/
Sleep 6s
Type "ip -details route show exact 198.51.100.0/24"
Enter
Wait+Screen /proto 250/
Sleep 6s
Screenshot artifacts/rib-fib.png

Hide
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh rib-fib recap"
Enter
Show
Sleep 9s
