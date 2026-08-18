Output artifacts/host-inventory.webm
Source common.tape

Hide
Type "clear"
Enter
Type "bash /src/demos/terminal/cards.sh host-inventory intro"
Enter
Show
Wait+Screen /====/
Sleep 500ms
Screenshot artifacts/host-inventory.png

Hide
Type "clear"
Enter
Show
Type "ze show host kernel | ze pipe yaml"
Enter
Wait+Screen /architecture:/
Sleep 7s
Type "clear"
Enter
Type "ze show host cpu | ze pipe yaml"
Enter
Wait+Screen /cores:/
Sleep 7s
Type "clear"
Enter
Type "ze show host memory | ze pipe yaml"
Enter
Wait+Screen /total-bytes:/
Sleep 7s
Sleep 2s
