#!/bin/sh
# pppd/rp-pppoe client entrypoint. Loads the kernel PPP modules from the shared
# host /lib/modules, then idles: the scenario drives pppd with `docker exec` so
# it can dial more than once with different credentials.
set -e

modprobe ppp_generic 2>/dev/null || true
modprobe pppoe 2>/dev/null || true

mkdir -p /etc/ppp /var/log/ppp

# `tail -f /dev/null` rather than `sleep infinity`: busybox sleep accepts
# "infinity" only on newer builds, and a wrong argument here would exit the
# container before the scenario ever dials.
exec tail -f /dev/null
