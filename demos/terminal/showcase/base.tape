Type "ze-demo card showcase bgp"
Enter
Wait+Screen /====/
Sleep 5s
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Type "exit"
Enter
Wait+Screen /ze>/
Type "monitor bgp"
Enter
Wait+Screen /established/
Sleep 3s
Type "s"
Wait+Screen /ASN \^/
Sleep 3s
Type "q"
Wait+Screen /ze>/
Type "exit"
Enter
Wait+Screen /\$ /
