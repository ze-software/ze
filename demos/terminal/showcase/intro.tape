Type "ze-demo card showcase intro"
Enter
Wait+Screen /====/
Sleep 6s
Type "ze exabgp migrate /src/demos/terminal/showcase/exabgp.conf"
Enter
Wait+Screen /peer transit/
Wait+Screen /local 65000/
Wait+Screen /remote 65001/
Wait+Screen /receive-hold-time 180/
Sleep 5s
