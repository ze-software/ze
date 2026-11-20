Type "ze-demo card showcase bgp"
Enter
Wait+Screen /====/
Sleep 2500ms
Type "sshpass -e ssh ze-demo"
Enter
Wait+Screen /ze#/
Type "exit"
Enter
Wait+Screen /ze>/
Type "monitor bgp"
Enter
Wait+Screen /established/
Sleep 1500ms
Type "s"
Wait+Screen /ASN \^/
Sleep 1500ms
Type "q"
Wait+Screen /ze>/
Type "exit"
Enter
Wait+Screen /\$ /
