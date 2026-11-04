# The Customer source's 10.10.0.0/24 may reach FRR's advertised Customer.
# The positive route fails if Ze never observes the roleless destination OPEN.
option=tcp_connections:value=1
option=bind:value=0.0.0.0
option=timeout:value=180s
option=linger:value=true
option=open:value=add-capability:code=9:hex=03
# AS65004, NEXT_HOP 172.30.0.9, no OTC; followed by End-of-RIB.
action=send:conn=1:seq=1:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF002F02000000144001010040020602010000FDEC400304AC1E0009180A0A00
action=send:conn=1:seq=2:hex=FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000
