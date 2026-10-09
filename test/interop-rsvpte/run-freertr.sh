#!/bin/sh
# Starts one freeRouter on this container's eth0 for the rsvpte interop suite
# (internal/le/interoplab/rsvpte). The scenario mounts rtr-hw.txt and
# rtr-sw.txt under /etc/freertr. freeRouter owns its own IPv4 stack and MAC on
# the shared segment, so eth0 is promiscuous and rawInt.bin carries every frame
# between eth0 and the jar over loopback UDP. tcpdump records the RSVP this
# peer sends and receives in /run/fr/rsvp.txt, which the checker reads, with
# MPLS frames too, because a PATH carried through a bypass LSP is labelled.
set -eu
mkdir -p /run/fr
ip link set eth0 promisc on
tcpdump -i eth0 -nn -l -vvv 'ip proto 46 or mpls' > /run/fr/rsvp.txt 2> /run/fr/tcpdump.err &
java -Xms32m -Xmx256m -jar /peer/rtr.jar routers /etc/freertr/rtr-hw.txt /etc/freertr/rtr-sw.txt > /run/fr/console.txt 2>&1 &
# rawInt's connected UDP reader exits on ICMP port-unreachable, so the jar's
# socket on 127.0.0.1:26001 (0x6591) MUST be bound before rawInt starts.
timeout 60 sh -c "until grep -q '0100007F:6591' /proc/net/udp; do sleep 1; done"
exec /peer/rawInt.bin eth0 26002 127.0.0.1 26001 127.0.0.1
