# Runs in FRR's container before FRR starts (prepare.go, prestartSuffix).
#
# The same two VLANs as ze-prestart.sh, and FRR's ONE link-local on both, so
# the address alone cannot tell the two sessions apart: only the link can.
ip link add link eth0 name eth1 type vlan id 101
ip link add link eth0 name eth2 type vlan id 102
ip -6 address add @FRR_LINK_LOCAL@/64 dev eth1 nodad
ip -6 address add @FRR_LINK_LOCAL@/64 dev eth2 nodad
ip link set eth1 up
ip link set eth2 up
