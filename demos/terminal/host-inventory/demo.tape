Output artifacts/host-inventory.webm
Source common.tape

Hide
Type "clear"
Enter
Type "ze-demo card host-inventory intro"
Enter
Show
Wait+Screen /====/
Sleep 250ms
Screenshot artifacts/host-inventory.png

Hide
Type "clear"
Enter
Show
Type "ze show host kernel | ze pipe yaml"
Enter
Wait+Screen /architecture:/
Sleep 3500ms
Type "clear"
Enter
Type "ze show host cpu | ze pipe yaml"
Enter
Wait+Screen /cores:/
Sleep 3500ms
Type "clear"
Enter
Type "ze show host memory | ze pipe yaml"
Enter
Wait+Screen /total-bytes:/
Sleep 3500ms
Sleep 1s
