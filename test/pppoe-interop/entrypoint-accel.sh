#!/bin/sh
# accel-ppp PPPoE server entrypoint. Loads the kernel PPP/PPPoE modules from
# the shared host /lib/modules, then runs accel-pppd in the foreground so the
# container stays alive and its logs reach `docker logs`.
set -e

modprobe ppp_generic 2>/dev/null || true
modprobe pppoe 2>/dev/null || true

mkdir -p /var/run /var/log /etc/accel-ppp

exec accel-pppd -c /etc/accel-ppp.conf -p /var/run/accel-ppp.pid
