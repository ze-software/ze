# The only source announcements occur after FRR is a live forwarding target.
# The two prefixes share the same UPDATE and attributes: 10.73.1.0/24 subject,
# 10.73.2.0/24 permitted fence. Nothing is sent during reject/accept reloads.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=240s
option=linger:value=true
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
expect=bgp:conn=1:seq=2:announced=180A49FE
action=send:conn=1:seq=3:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003A020000001B4001010040020602010000FDEC400304AC1E0009C00804FDEC0063180A4901180A4902
