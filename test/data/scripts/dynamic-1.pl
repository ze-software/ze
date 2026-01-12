#!/usr/bin/perl

use strict;
use warnings;

# Ignore Control C
# allow exabgp to send us a SIGTERM when it is time
$SIG{'INT'} = sub {};

# make STDOUT unbuffered
select STDOUT; $| = 1;

while (1) {
    # commands are case sensitive
    print "announce flow route {\\\\n match {\\\\n source 10.0.0.1/32;\\\\n destination 1.2.3.4/32;\\\\n }\\\\n then {\\\\n discard;\\\\n }\\\\n }\\\\n";
    sleep 10;
    print "update text nhop set 10.0.0.1 nlri ipv4/unicast add 192.0.2.1/32";
    sleep 10;
    print "update text nlri ipv4/unicast del 192.0.2.1/32";
    sleep 10;
    print "withdraw flow route {\\\\n match {\\\\n source 10.0.0.1/32;\\\\n destination 1.2.3.4/32;\\\\n }\\\\n then {\\\\n discard;\\\\n }\\\\n }\\\\n";
    sleep 10;
}
